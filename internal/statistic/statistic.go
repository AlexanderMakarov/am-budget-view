package statistic

import (
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"time"

	"github.com/AlexanderMakarov/am-budget-view/internal/config"
	"github.com/AlexanderMakarov/am-budget-view/internal/i18n"
	"github.com/AlexanderMakarov/am-budget-view/internal/model"
)

// FormatTransaction returns a human-readable string for a Transaction.
func FormatTransaction(t *model.Transaction) string {
	return fmt.Sprint(i18n.T("Transaction date amount details", "date", t.Date, "amount", t.Amount, "details", t.Details))
}

// FormatJournalEntry returns a human-readable string for a JournalEntry.
func FormatJournalEntry(je *model.JournalEntry) string {
	direction := i18n.T("Income")
	if je.IsExpense {
		direction = i18n.T("Expense")
	}
	amounts := ""
	currencies := []string{}
	for currency := range je.Amounts {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)
	for _, currency := range currencies {
		amount := je.Amounts[currency]
		amounts += fmt.Sprintf("\t%s %s (%d)", amount.Amount.StringNoIndent(), currency, amount.ConversionPrecision)
	}
	return fmt.Sprintf(
		"%s\t%s\t%s %s\t%s\t%s->%s\t%s\t%s\t'%s'%s",
		je.Date.Format(model.OutputDateFormat),
		direction,
		je.AccountCurrencyAmount.String(),
		je.AccountCurrency,
		je.Category,
		je.FromAccount,
		je.ToAccount,
		je.Source.TypeName,
		je.Source.FilePath,
		je.Details,
		amounts,
	)
}

// FormatIntervalStatistic returns a human-readable string for an IntervalStatistic.
func FormatIntervalStatistic(s *model.IntervalStatistic) string {
	income := MapOfGroupsToStringFull(s.Income, true)
	expense := MapOfGroupsToStringFull(s.Expense, true)
	return i18n.T("Statistics_format",
		"start", s.Start,
		"end", s.End,
		"currency", s.Currency,
		"nIncome", len(income),
		"sumIncome", model.MapOfGroupsSum(s.Income),
		"detailsIncome", income,
		"nExpense", len(expense),
		"sumExpense", model.MapOfGroupsSum(s.Expense),
		"detailsExpense", expense,
	)
}

const UnknownGroupName = "Unknown"

// MapOfGroupsToStringFull converts map of Group-s to human readable string.
// withJournalEntries parameter allows to output all journal entries for each group.
func MapOfGroupsToStringFull(mapOfGroups map[string]*model.Group, withJournalEntries bool) []string {
	groupList := make(model.GroupList, 0, len(mapOfGroups))
	for _, group := range mapOfGroups {
		groupList = append(groupList, group)
	}

	sort.Sort(groupList)

	groupStrings := []string{}
	for _, group := range groupList {
		if group.Total.Cents == 0 {
			continue
		}
		if withJournalEntries {
			journalEntryStrings := make([]string, len(group.JournalEntries))
			for j, je := range group.JournalEntries {
				journalEntryStrings[j] = FormatJournalEntry(&je)
			}
			groupStrings = append(groupStrings,
				i18n.T("groupName total nTransactions details",
					"groupName", group.Name,
					"total", group.Total,
					"nTransactions", len(journalEntryStrings),
					"details", journalEntryStrings,
				),
			)
		} else {
			groupStrings = append(groupStrings,
				i18n.T("groupName total",
					"groupName", group.Name,
					"total", group.Total,
				),
			)
		}
	}
	return groupStrings
}

// MapOfGroupsToString converts map of Group-s to list of human readable strings.
func MapOfGroupsToString(mapOfGroups map[string]*model.Group) []string {
	return MapOfGroupsToStringFull(mapOfGroups, false)
}

// DumpIntervalStatistics dumps IntervalStatistic to io.Writer.
// If currency is not empty string then only statistics for this currency will be dumped.
func DumpIntervalStatistics(intervalStatistics map[string]*model.IntervalStatistic, writer io.Writer, currency string, isDetailed bool) error {
	if currency == "" {
		currenciesSorted := make([]string, 0, len(intervalStatistics))
		for currency := range intervalStatistics {
			currenciesSorted = append(currenciesSorted, currency)
		}
		sort.Strings(currenciesSorted)
		for _, currency := range currenciesSorted {
			DumpIntervalStatistic(intervalStatistics[currency], writer, currency, isDetailed)
		}
	} else {
		if stat, ok := intervalStatistics[currency]; ok {
			DumpIntervalStatistic(stat, writer, currency, isDetailed)
		} else {
			return errors.New(i18n.T("no statistics for c currency", "c", currency))
		}
	}
	return nil
}

// DumpIntervalStatistic dumps IntervalStatistic to io.Writer.
func DumpIntervalStatistic(intervalStatistic *model.IntervalStatistic, writer io.Writer, currency string, isDetailed bool) {
	if isDetailed {
		fmt.Fprint(writer, i18n.T("c amounts\n stats\n", "c", currency, "stats", FormatIntervalStatistic(intervalStatistic)))
		return
	}
	income := MapOfGroupsToString(intervalStatistic.Income)
	expense := MapOfGroupsToString(intervalStatistic.Expense)
	fmt.Fprintln(writer,
		i18n.T("Statistics_format",
			"start", intervalStatistic.Start,
			"end", intervalStatistic.End,
			"currency", currency,
			"nIncome", len(income),
			"sumIncome", model.MapOfGroupsSum(intervalStatistic.Income),
			"detailsIncome", income,
			"nExpense", len(expense),
			"sumExpense", model.MapOfGroupsSum(intervalStatistic.Expense),
			"detailsExpense", expense,
		),
	)
}

// IntervalStatisticsBuilder builds IntervalStatistic from JournalEntry-s.
type IntervalStatisticsBuilder interface {
	HandleJournalEntry(je model.JournalEntry, start, end time.Time) error
	GetIntervalStatistics() map[string]*model.IntervalStatistic
}

// GroupExtractorByCategories is IntervalStatisticsBuilder which
// converts JournalEntry-s into groups by category and ignores transactions to my accounts in "Total".
type GroupExtractorByCategories struct {
	intervalStats map[string]*model.IntervalStatistic
	myAccounts    map[string]struct{}
}

func (s GroupExtractorByCategories) HandleJournalEntry(je model.JournalEntry, start, end time.Time) error {
	for _, amount := range je.Amounts {
		currency := amount.Currency
		stat, ok := s.intervalStats[currency]
		if !ok {
			stat = &model.IntervalStatistic{
				Currency: currency,
				Start:    start,
				End:      end,
				Income:   make(map[string]*model.Group),
				Expense:  make(map[string]*model.Group),
			}
			s.intervalStats[currency] = stat
		}
		if je.IsExpense {
			group, exists := stat.Expense[je.Category]
			if !exists {
				group = &model.Group{
					Name:  je.Category,
					Total: model.MoneyWith2DecimalPlaces{Cents: 0},
				}
				stat.Expense[je.Category] = group
			}
			group.JournalEntries = append(group.JournalEntries, je)
			if _, ok := s.myAccounts[je.ToAccount]; !ok {
				group.Total.Cents += amount.Amount.Cents
			}
		} else {
			group, exists := stat.Income[je.Category]
			if !exists {
				group = &model.Group{
					Name:  je.Category,
					Total: model.MoneyWith2DecimalPlaces{Cents: 0},
				}
				stat.Income[je.Category] = group
			}
			group.JournalEntries = append(group.JournalEntries, je)
			if _, ok := s.myAccounts[je.FromAccount]; !ok {
				group.Total.Cents += amount.Amount.Cents
			}
		}
	}
	return nil
}

func (s GroupExtractorByCategories) GetIntervalStatistics() map[string]*model.IntervalStatistic {
	return s.intervalStats
}

type StatisticBuilderFactory func(start, end time.Time) IntervalStatisticsBuilder

// NewStatisticBuilderByCategories returns a factory that builds GroupExtractorByCategories.
func NewStatisticBuilderByCategories(accounts map[string]*model.AccountStatistics, cfg *config.Config) (StatisticBuilderFactory, error) {
	myAccounts := make(map[string]struct{})
	for _, account := range accounts {
		if account.IsTransactionAccount {
			myAccounts[account.Number] = struct{}{}
		}
	}
	if cfg != nil {
		for _, acc := range cfg.MyAccounts {
			if acc == "" {
				continue
			}
			myAccounts[acc] = struct{}{}
		}
	}
	keys := make([]string, 0, len(myAccounts))
	for k := range myAccounts {
		keys = append(keys, k)
	}
	log.Println(i18n.T("My accounts (will be ignored for totals): accounts", "accounts", keys))

	return func(start, end time.Time) IntervalStatisticsBuilder {
		return GroupExtractorByCategories{
			intervalStats: make(map[string]*model.IntervalStatistic),
			myAccounts:    myAccounts,
		}
	}, nil
}

// BuildMonthlyStatistics builds list of IntervalStatistic per each month from provided journal entries.
func BuildMonthlyStatistics(
	journalEntries []model.JournalEntry,
	statisticBuilderFactory StatisticBuilderFactory,
	monthStart uint,
	timeZone *time.Location,
) ([]map[string]*model.IntervalStatistic, error) {

	result := make([]map[string]*model.IntervalStatistic, 0)
	var statBuilder IntervalStatisticsBuilder

	start := time.Date(journalEntries[0].Date.Year(), journalEntries[0].Date.Month(),
		int(monthStart), 0, 0, 0, 0, timeZone)
	end := start.AddDate(0, 1, 0).Add(-1 * time.Nanosecond)
	statBuilder = statisticBuilderFactory(start, end)

	for _, je := range journalEntries {
		if je.Date.After(end) {
			result = append(result, statBuilder.GetIntervalStatistics())
			start = time.Date(je.Date.Year(), je.Date.Month(), int(monthStart), 0, 0, 0, 0, timeZone)
			end = start.AddDate(0, 1, 0).Add(-1 * time.Nanosecond)
			statBuilder = statisticBuilderFactory(start, end)
		}
		if err := statBuilder.HandleJournalEntry(je, start, end); err != nil {
			return nil, err
		}
	}

	result = append(result, statBuilder.GetIntervalStatistics())
	return result, nil
}
