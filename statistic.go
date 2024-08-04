package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"log"
)

func (t *Transaction) String() string {
	return fmt.Sprintf("Transaction %s %s %s", t.Date.Format(OutputDateFormat), t.Amount, t.Details)
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

// GroupList structure to sort groups by `TotalAmount2DigitAfterDot` descending.
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
// `withTransactions` parameter allows to output all transactions for the each group.
func MapOfGroupsToStringFull(mapOfGroups map[string]*Group, withTransactions bool) []string {
	groupList := make(GroupList, 0, len(mapOfGroups))
	for _, group := range mapOfGroups {
		groupList = append(groupList, group)
	}

	// Sort the slice by TotalAmount2DigitAfterDot.
	sort.Sort(groupList)

	groupStrings := []string{}
	for _, group := range groupList {
		if withTransactions {
			transStrings := make([]string, len(group.Transactions))
			for j, t := range group.Transactions {
				transStrings[j] = t.String()
			}
			groupStrings = append(groupStrings,
				fmt.Sprintf(
					"\n    %-35s: %s, from %d transaction(s):\n      %s",
					group.Name,
					group.Total,
					len(transStrings),
					strings.Join(transStrings, "\n      "),
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
	return fmt.Sprintf("Statistics for %s..%s:\n  Income (%d, sum=%s):%s\n  Expenses (%d, sum=%s):%s\n",
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

// IntervalStatisticsBuilder builds `IntervalStatistic` from `Transaction-s`.
type IntervalStatisticsBuilder interface {

	// HandleTransaction updates inner `MonthStatistics` object with transaction details.
	// The main purpose is to choose right `Group` instance to add data into.
	HandleTransaction(trans Transaction) error

	// GetIntervalStatistic returns `IntervalStatistic` assembled so far.
	GetIntervalStatistic() *IntervalStatistic
}

const UnknownGroupName = "unknown"

// GroupExtractorByDetailsSubstrings is [main.IntervalStatisticsBuilder] which uses
// `Transaction.Details` field to choose right group. Logic is following:
//  1. Find is group for expenses of incomes.
//  2. Search group in `substringsToGroupName` field. If there are such then update it.
//  3. Otherwise check isGroupAllUnknown value:
//  4. If `false` then create new group with name equal to `Transaction.Details` field
//  5. If `true` then add into single group with name from `UnknownGroupName` constant.
type GroupExtractorByDetailsSubstrings struct {
	intervalStats          *IntervalStatistic
	groupNamesToSubstrings map[string][]string
	substringsToGroupName  map[string]string
	isGroupAllUnknown      bool
	ignoreSubstrings       []string
}

func (s GroupExtractorByDetailsSubstrings) HandleTransaction(trans Transaction) error {

	// Choose map of groups to operate on.
	var mapOfGroups map[string]*Group
	if trans.IsExpense {
		mapOfGroups = s.intervalStats.Expense
	} else {
		mapOfGroups = s.intervalStats.Income
	}

	// First check that need to ignore transaction.
	for _, substring := range s.ignoreSubstrings {
		if strings.Contains(trans.Details, substring) {
			return nil
		}
	}

	// Try to find user-defined group in configuration and add transaction to it.
	found := false
	for substring, groupName := range s.substringsToGroupName {
		if strings.Contains(trans.Details, substring) {
			group, exists := mapOfGroups[groupName]
			if !exists {
				// If the group doesn't exist in the map, create a new one.
				group = &Group{
					Name:         groupName,
					Total:        MoneyWith2DecimalPlaces{},
					Transactions: []Transaction{},
				}
				mapOfGroups[groupName] = group
			}
			group.Transactions = append(group.Transactions, trans)
			group.Total.int += trans.Amount.int
			found = true
			break
		}
	}

	// Otherwise add transaction to either "unknown" or personal group.
	if !found {
		// Choose name of group to add transaction into.
		var groupName string
		if s.isGroupAllUnknown {
			groupName = UnknownGroupName
		} else {
			groupName = trans.Details
		}

		// Check group exists or create a new one. Add transaction to group.
		if group, exists := mapOfGroups[groupName]; exists {
			group.Total.int += trans.Amount.int
			group.Transactions = append(group.Transactions, trans)
		} else {
			newGroup := Group{
				Name:         groupName,
				Total:        trans.Amount,
				Transactions: []Transaction{trans},
			}
			mapOfGroups[newGroup.Name] = &newGroup
		}
	}

	return nil
}

func (s GroupExtractorByDetailsSubstrings) GetIntervalStatistic() *IntervalStatistic {
	return s.intervalStats
}

type StatisticBuilderFactory func(start, end time.Time) IntervalStatisticsBuilder

// NewStatisticBuilderByDetailsSubstrings returns
// [github.com/AlexanderMakarov/am-budget-view.main.GroupExtractorBuilder] which builds
// [github.com/AlexanderMakarov/am-budget-view.main.groupExtractorByDetailsSubstrings] in a safe way.
func NewStatisticBuilderByDetailsSubstrings(
	groupNamesToSubstrings map[string][]string,
	isGroupAllUnknownTransactions bool,
	ignoreSubstrings []string,
) (StatisticBuilderFactory, error) {

	// Invert groupNamesToSubstrings and check for duplicates.
	substringsToGroupName := map[string]string{}
	for name, substrings := range groupNamesToSubstrings {
		for _, substring := range substrings {
			if group, exist := substringsToGroupName[substring]; exist {
				return nil, fmt.Errorf("'%s' is duplicated in '%s' and in previous '%s'",
					substring, name, group)
			}
			substringsToGroupName[substring] = name
		}
	}
	log.Printf("Going to separate transactions by %d named groups from %d substrings",
		len(groupNamesToSubstrings), len(substringsToGroupName))

	return func(start, end time.Time) IntervalStatisticsBuilder {

		// Return new groupExtractorByDetailsSubstrings.
		return GroupExtractorByDetailsSubstrings{
			intervalStats: &IntervalStatistic{
				Start:   start,
				End:     end,
				Income:  make(map[string]*Group),
				Expense: make(map[string]*Group),
			},
			groupNamesToSubstrings: groupNamesToSubstrings,
			substringsToGroupName:  substringsToGroupName,
			isGroupAllUnknown:      isGroupAllUnknownTransactions,
			ignoreSubstrings:       ignoreSubstrings,
		}
	}, nil
}

// BuildMonthlyStatistic builds list of
// [github.com/AlexanderMakarov/am-budget-view.main.IntervalStatistic]
// per each month from provided transactions.
func BuildMonthlyStatistic(
	transactions []Transaction,
	statisticBuilderFactory StatisticBuilderFactory,
	monthStart uint,
	timeZone *time.Location,
) ([]*IntervalStatistic, error) {

	// Sort transactions.
	sort.Sort(TransactionList(transactions))

	var stats []*IntervalStatistic
	var statBuilder IntervalStatisticsBuilder

	// Get first month boundaries from the first transaction. Build first month statistics.
	start := time.Date(transactions[0].Date.Year(), transactions[0].Date.Month(),
		int(monthStart), 0, 0, 0, 0, timeZone)
	end := start.AddDate(0, 1, 0).Add(-1 * time.Nanosecond)
	statBuilder = statisticBuilderFactory(start, end)

	// Iterate through all the transactions.
	for _, trans := range transactions {

		// Check if this transaction is part of the new month.
		if trans.Date.After(end) {

			// Save previous month statistic if there is one.
			stats = append(stats, statBuilder.GetIntervalStatistic())

			// Calculate start and end of the next month.
			start = time.Date(trans.Date.Year(), trans.Date.Month(), int(monthStart), 0, 0, 0, 0, timeZone)
			end = start.AddDate(0, 1, 0).Add(-1 * time.Nanosecond)
			statBuilder = statisticBuilderFactory(start, end)
		}

		// Handle transaction.
		if err := statBuilder.HandleTransaction(trans); err != nil {
			return nil, err
		}
	}

	// Add last IntervalStatistic if need.
	lastStatistic := statBuilder.GetIntervalStatistic()
	if len(lastStatistic.Expense) > 0 || len(lastStatistic.Income) > 0 {
		stats = append(stats, lastStatistic)
	}
	return stats, nil
}
