package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (t *Transaction) String() string {
	return fmt.Sprintf("Transaction %s %s %s", t.Date.Format(OutputDateFormat), t.Amount, t.Details)
}

func (je *JournalEntry) String() string {
	direction := "Income "
	if je.IsExpense {
		direction = "Expense"
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
		je.Date.Format(OutputDateFormat),
		direction,
		je.AccountCurrencyAmount.String(),
		je.AccountCurrency,
		je.Category,
		je.FromAccount,
		je.ToAccount,
		je.SourceType,
		je.Source,
		je.Details,
		amounts,
	)
}

func (m MoneyWith2DecimalPlaces) String() string {
	dollars := m.int / 100
	cents := m.int % 100
	dollarString := strconv.Itoa(dollars)
	for i := len(dollarString) - 3; i > 0; i -= 3 {
		dollarString = dollarString[:i] + "," + dollarString[i:]
	}
	return fmt.Sprintf("%9s.%02d", dollarString, cents)
}

// GroupList structure to sort groups by `MoneyWith2DecimalPlaces` descending.
type GroupList []*Group

func (g GroupList) Len() int {
	return len(g)
}

func (g GroupList) Less(i, j int) bool {
	return g[i].Total.int > g[j].Total.int
}

func (g GroupList) Swap(i, j int) {
	g[i], g[j] = g[j], g[i]
}

// TransactionList structure to sort transaction by `Date` ascending.
type TransactionList []Transaction

func (g TransactionList) Len() int {
	return len(g)
}

func (g TransactionList) Less(i, j int) bool {
	return g[i].Date.Before(g[j].Date)
}

func (g TransactionList) Swap(i, j int) {
	g[i], g[j] = g[j], g[i]
}

// MapOfGroupsToStringFull converts map of `Group`-s to human readable string.
// `withJournalEntries` parameter allows to output all journal entries for the each group.
func MapOfGroupsToStringFull(mapOfGroups map[string]*Group, withJournalEntries bool) []string {
	groupList := make(GroupList, 0, len(mapOfGroups))
	for _, group := range mapOfGroups {
		groupList = append(groupList, group)
	}

	// Sort the slice by TotalAmount2DigitAfterDot.
	sort.Sort(groupList)

	groupStrings := []string{}
	for _, group := range groupList {
		// Skip groups with zero total.
		if group.Total.int == 0 {
			continue
		}
		// Check if need to output journal entries.
		if withJournalEntries {
			journalEntryStrings := make([]string, len(group.JournalEntries))
			for j, je := range group.JournalEntries {
				journalEntryStrings[j] = je.String()
			}
			groupStrings = append(groupStrings,
				fmt.Sprintf(
					"\n    %-35s: %s, from %d transaction(s):\n      %s",
					group.Name,
					group.Total,
					len(journalEntryStrings),
					strings.Join(journalEntryStrings, "\n      "),
				),
			)
		} else {
			groupStrings = append(groupStrings,
				fmt.Sprintf(
					"\n    %-35s: %s",
					group.Name,
					group.Total,
				),
			)
		}
	}
	return groupStrings
}

// MapOfGroupsToStringFull converts map of `Group`-s to human readable string.
func MapOfGroupsToString(mapOfGroups map[string]*Group) []string {
	return MapOfGroupsToStringFull(mapOfGroups, false)
}

func (s *IntervalStatistic) String() string {
	income := MapOfGroupsToStringFull(s.Income, true)
	expense := MapOfGroupsToStringFull(s.Expense, true)
	return fmt.Sprintf(
		"Statistics for %s..%s:\n  Income  (total %2d groups, filtered sum %s):%s\n  Expenses (total %2d groups, filt-ed sum %s):%s\n",
		s.Start.Format(OutputDateFormat),
		s.End.Format(OutputDateFormat),
		len(income),
		MapOfGroupsSum(s.Income),
		strings.Join(income, ""),
		len(s.Expense),
		MapOfGroupsSum(s.Expense),
		strings.Join(expense, ""),
	)
}

// MapOfGroupsSum returns sum from all groups.
func MapOfGroupsSum(mapOfGroups map[string]*Group) MoneyWith2DecimalPlaces {
	sum := MoneyWith2DecimalPlaces{}
	for _, group := range mapOfGroups {
		sum.int += group.Total.int
	}
	return sum
}

// DumpIntervalStatistics dumps `IntervalStatistic` to `io.Writer`.
// If `currency` is not empty string then only statistics for this currency will be dumped.
func DumpIntervalStatistics(intervalStatistics map[string]*IntervalStatistic, writer io.Writer, currency string, isDetailed bool) error {
	if currency == "" {
		// If currency is not provided then dump statistics for all currencies alphabetically.
		currenciesSorted := make([]string, 0, len(intervalStatistics))
		for currency := range intervalStatistics {
			currenciesSorted = append(currenciesSorted, currency)
		}
		sort.Strings(currenciesSorted)
		for _, currency := range currenciesSorted {
			DumpIntervalStatistic(intervalStatistics[currency], writer, currency, isDetailed)
		}
	} else {
		// If currency is provided then dump statistics only for this currency.
		if stat, ok := intervalStatistics[currency]; ok {
			DumpIntervalStatistic(stat, writer, currency, isDetailed)
		} else {
			return fmt.Errorf("no statistics for currency %s", currency)
		}
	}
	return nil
}

// DumpIntervalStatistic dumps `IntervalStatistic` to `io.Writer`.
// If `isDetailed` is true then uses `String()` method.
func DumpIntervalStatistic(intervalStatistic *IntervalStatistic, writer io.Writer, currency string, isDetailed bool) {
	// If need detailed output then use `String()` method.
	if isDetailed {
		fmt.Fprintf(writer, "%s amounts:\n", currency)
		fmt.Fprintf(writer, "%s\n", intervalStatistic)
		return
	}
	// Otherwise use `MapOfGroupsToString` to dump income and expense.
	income := MapOfGroupsToString(intervalStatistic.Income)
	expense := MapOfGroupsToString(intervalStatistic.Expense)
	fmt.Fprintf(writer,
		"Statistics for %s..%s (in %s):\n  Income  (total %2d groups, filtered sum %s):%s\n  Expenses (total %2d groups, filt-ed sum %s):%s\n",
		intervalStatistic.Start.Format(OutputDateFormat),
		intervalStatistic.End.Format(OutputDateFormat),
		currency,
		len(income),
		MapOfGroupsSum(intervalStatistic.Income),
		strings.Join(income, ""),
		len(expense),
		MapOfGroupsSum(intervalStatistic.Expense),
		strings.Join(expense, ""),
	)
}

// IntervalStatisticsBuilder builds `IntervalStatistic` from `JournalEntry`-s.
type IntervalStatisticsBuilder interface {

	// HandleJournalEntry updates inner state with JournalEntry details.
	// The main purpose is to choose right `Group` instance to add data into.
	HandleJournalEntry(je JournalEntry, start, end time.Time) error

	// GetIntervalStatistics returns map of `IntervalStatistic` per each currency assembled so far.
	GetIntervalStatistics() map[string]*IntervalStatistic
}

const UnknownGroupName = "unknown"

// GroupExtractorByCategories is [main.IntervalStatisticsBuilder] which
// converts JournalEntry-s into groups by category and ignores transactions to my accounts in "Total".
type GroupExtractorByCategories struct {
	intervalStats map[string]*IntervalStatistic
	myAccounts    map[string]struct{}
}

func (s GroupExtractorByCategories) HandleJournalEntry(je JournalEntry, start, end time.Time) error {
	for _, amount := range je.Amounts {
		currency := amount.Currency
		stat, ok := s.intervalStats[currency]
		if !ok {
			stat = &IntervalStatistic{
				Currency: currency,
				Start:    start,
				End:      end,
				Income:   make(map[string]*Group),
				Expense:  make(map[string]*Group),
			}
			s.intervalStats[currency] = stat
		}
		if je.IsExpense {
			group, exists := stat.Expense[je.Category]
			if !exists {
				group = &Group{
					Name:  je.Category,
					Total: MoneyWith2DecimalPlaces{int: 0},
				}
				stat.Expense[je.Category] = group
			}
			group.JournalEntries = append(group.JournalEntries, je)
			// Add to total only if destination account is not mine.
			if _, ok := s.myAccounts[je.ToAccount]; !ok {
				group.Total.int += amount.Amount.int
			}
		} else {
			group, exists := stat.Income[je.Category]
			if !exists {
				group = &Group{
					Name:  je.Category,
					Total: MoneyWith2DecimalPlaces{int: 0},
				}
				stat.Income[je.Category] = group
			}
			group.JournalEntries = append(group.JournalEntries, je)
			// Add to total only if source account is not mine.
			if _, ok := s.myAccounts[je.FromAccount]; !ok {
				group.Total.int += amount.Amount.int
			}
		}
	}

	return nil
}

func (s GroupExtractorByCategories) GetIntervalStatistics() map[string]*IntervalStatistic {
	return s.intervalStats
}

type StatisticBuilderFactory func(start, end time.Time, accounts map[string]*AccountFromTransactions) IntervalStatisticsBuilder

// NewStatisticBuilderByCategories returns
// [github.com/AlexanderMakarov/am-budget-view.main.GroupExtractorBuilder] which builds
// [github.com/AlexanderMakarov/am-budget-view.main.groupExtractorByCategories] in a safe way.
func NewStatisticBuilderByCategories() (StatisticBuilderFactory, error) {
	return func(start, end time.Time, accounts map[string]*AccountFromTransactions) IntervalStatisticsBuilder {
		myAccounts := make(map[string]struct{})
		for _, account := range accounts {
			if account.IsTransactionAccount {
				myAccounts[account.Number] = struct{}{}
			}
		}
		fmt.Printf("My accounts (will be ignored for totals): %v\n", myAccounts)
		return GroupExtractorByCategories{
			intervalStats: make(map[string]*IntervalStatistic),
			myAccounts:    myAccounts,
		}
	}, nil
}

// BuildMonthlyStatistics builds list of
// [github.com/AlexanderMakarov/am-budget-view.main.IntervalStatistic]
// per each month from provided journal entries.
func BuildMonthlyStatistics(
	journalEntries []JournalEntry,
	accounts map[string]*AccountFromTransactions,
	statisticBuilderFactory StatisticBuilderFactory,
	monthStart uint,
	timeZone *time.Location,
) ([]map[string]*IntervalStatistic, error) {

	result := make([]map[string]*IntervalStatistic, 0)
	var statBuilder IntervalStatisticsBuilder

	// Get first month boundaries from the first transaction. Build first month statistics.
	start := time.Date(journalEntries[0].Date.Year(), journalEntries[0].Date.Month(),
		int(monthStart), 0, 0, 0, 0, timeZone)
	end := start.AddDate(0, 1, 0).Add(-1 * time.Nanosecond)
	statBuilder = statisticBuilderFactory(start, end, accounts)

	// Iterate through all the journal entries.
	for _, je := range journalEntries {

		// Check if this transaction is part of the new month.
		if je.Date.After(end) {

			// Save previous month statistic if there is one.
			result = append(result, statBuilder.GetIntervalStatistics())

			// Calculate start and end of the next month.
			start = time.Date(je.Date.Year(), je.Date.Month(), int(monthStart), 0, 0, 0, 0, timeZone)
			end = start.AddDate(0, 1, 0).Add(-1 * time.Nanosecond)
			statBuilder = statisticBuilderFactory(start, end, accounts)
		}

		// Handle/append journal entry.
		if err := statBuilder.HandleJournalEntry(je, start, end); err != nil {
			return nil, err
		}
	}

	// Add last IntervalStatistics if need.
	result = append(result, statBuilder.GetIntervalStatistics())

	return result, nil
}
