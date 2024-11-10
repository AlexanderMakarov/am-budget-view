package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

var (
	date1 = time.Date(2024, 10, 27, 0, 0, 0, 0, time.Local)
	date2 = date1.AddDate(0, 0, 1)
)

func Test_NewGroupExtractorByCategories(t *testing.T) {

	// Cases
	tests := []struct {
		name               string
		accounts           map[string]*AccountFromTransactions
		expectedMyAccounts map[string]struct{}
	}{
		{
			"no_accounts",
			map[string]*AccountFromTransactions{},
			map[string]struct{}{},
		},
		{
			"no_my_accounts",
			map[string]*AccountFromTransactions{
				"a": {Number: "a", IsTransactionAccount: false},
			},
			map[string]struct{}{},
		},
		{
			"many_my_accounts",
			map[string]*AccountFromTransactions{
				"a": {Number: "a", IsTransactionAccount: true},
				"b": {Number: "b", IsTransactionAccount: false},
				"c": {Number: "c", IsTransactionAccount: true},
			},
			map[string]struct{}{"a": {}, "c": {}},
		},
	}
	const testName = "NewGroupExtractorByCategories()"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Act
			builder, err := NewStatisticBuilderByCategories(tt.accounts)
			actualGE := builder(date1, date2)

			// Assert
			if err != nil {
				t.Errorf("%s failed: %#v", testName, err)
			}
			if actualGE == nil {
				t.Errorf("%s builder returned null", testName)
			}
			myAccounts := actualGE.(GroupExtractorByCategories).myAccounts
			if !reflect.DeepEqual(myAccounts, tt.expectedMyAccounts) {
				t.Errorf("%s builder set wrong myAccounts: expected=%+v, actual=%+v", testName,
					tt.expectedMyAccounts, myAccounts)
			}
		})
	}
}

func Test_groupExtractorByCategories_HandleJournalEntry(t *testing.T) {
	a1Mine := &AccountFromTransactions{Number: "a1", IsTransactionAccount: true}
	a1NotMine := &AccountFromTransactions{Number: "a1", IsTransactionAccount: false}
	a2NotMine := &AccountFromTransactions{Number: "a2", IsTransactionAccount: false}
	a3NotMine := &AccountFromTransactions{Number: "a3", IsTransactionAccount: false}
	jesVariousCategories := []JournalEntry{
		newUsdJE(1, false, "a", "a1", "a2"),
		newUsdJE(2, false, "a", "a3", "a1"),
		newUsdJE(3, true, "b", "a1", "a3"),
		newUsdJE(4, true, "b", "a2", "a3"),
		newUsdJE(5, true, "b", "a2", "a3"),
		newUsdJE(6, false, "b", "a1", "a2"),
		newUsdJE(7, false, "c", "a2", "a3"),
		newUsdJE(8, true, "c", "a1", "a3"),
		newUsdJE(9, false, "c", "a3", "a1"),
		newUsdJE(10, true, "c", "a2", "a1"),
		newUsdJE(11, false, "c", "a3", "a1"),
		newUsdJE(12, false, "e", "a2", "a1"),
		newUsdJE(13, true, "e", "a3", "a2"),
	}
	expectedVariousCategoriesFull := `USD amounts:
Statistics for        2024-10-27..2024-10-28 (in    USD):
  Income  (total  4 groups, filtered sum           0.48):
    c                                  :         0.27, from 3 transaction(s):
      2024-10-27	Income	        0.07 	c	a2->a3			'+7c'	0.07 USD (0)
      2024-10-27	Income	        0.09 	c	a3->a1			'+9c'	0.09 USD (0)
      2024-10-27	Income	        0.11 	c	a3->a1			'+11c'	0.11 USD (0)
    e                                  :         0.12, from 1 transaction(s):
      2024-10-27	Income	        0.12 	e	a2->a1			'+12e'	0.12 USD (0)
    b                                  :         0.06, from 1 transaction(s):
      2024-10-27	Income	        0.06 	b	a1->a2			'+6b'	0.06 USD (0)
    a                                  :         0.03, from 2 transaction(s):
      2024-10-27	Income	        0.01 	a	a1->a2			'+1a'	0.01 USD (0)
      2024-10-27	Income	        0.02 	a	a3->a1			'+2a'	0.02 USD (0)
  Expenses (total  3 groups, filt-ed sum           0.43):
    c                                  :         0.18, from 2 transaction(s):
      2024-10-27	Expense	        0.08 	c	a1->a3			'-8c'	0.08 USD (0)
      2024-10-27	Expense	        0.10 	c	a2->a1			'-10c'	0.10 USD (0)
    e                                  :         0.13, from 1 transaction(s):
      2024-10-27	Expense	        0.13 	e	a3->a2			'-13e'	0.13 USD (0)
    b                                  :         0.12, from 3 transaction(s):
      2024-10-27	Expense	        0.03 	b	a1->a3			'-3b'	0.03 USD (0)
      2024-10-27	Expense	        0.04 	b	a2->a3			'-4b'	0.04 USD (0)
      2024-10-27	Expense	        0.05 	b	a2->a3			'-5b'	0.05 USD (0)
`
	expectedVariousCategoriesA1Mine := `USD amounts:
Statistics for        2024-10-27..2024-10-28 (in    USD):
  Income  (total  3 groups, filtered sum           0.41):
    c                                  :         0.27, from 3 transaction(s):
      2024-10-27	Income	        0.07 	c	a2->a3			'+7c'	0.07 USD (0)
      2024-10-27	Income	        0.09 	c	a3->a1			'+9c'	0.09 USD (0)
      2024-10-27	Income	        0.11 	c	a3->a1			'+11c'	0.11 USD (0)
    e                                  :         0.12, from 1 transaction(s):
      2024-10-27	Income	        0.12 	e	a2->a1			'+12e'	0.12 USD (0)
    a                                  :         0.02, from 2 transaction(s):
      2024-10-27	Income	        0.01 	a	a1->a2			'+1a'	0.01 USD (0)
      2024-10-27	Income	        0.02 	a	a3->a1			'+2a'	0.02 USD (0)
  Expenses (total  3 groups, filt-ed sum           0.33):
    e                                  :         0.13, from 1 transaction(s):
      2024-10-27	Expense	        0.13 	e	a3->a2			'-13e'	0.13 USD (0)
    b                                  :         0.12, from 3 transaction(s):
      2024-10-27	Expense	        0.03 	b	a1->a3			'-3b'	0.03 USD (0)
      2024-10-27	Expense	        0.04 	b	a2->a3			'-4b'	0.04 USD (0)
      2024-10-27	Expense	        0.05 	b	a2->a3			'-5b'	0.05 USD (0)
    c                                  :         0.08, from 2 transaction(s):
      2024-10-27	Expense	        0.08 	c	a1->a3			'-8c'	0.08 USD (0)
      2024-10-27	Expense	        0.10 	c	a2->a1			'-10c'	0.10 USD (0)
`
	jesVariousCurrencies := []JournalEntry{
		{
			IsExpense:             true,
			Date:                  date1,
			SourceType:            "t1",
			Source:                "s1",
			Details:               "expense, all in USD",
			Category:              "a",
			FromAccount:           "a1",
			ToAccount:             "a2",
			AccountCurrency:       "USD",
			AccountCurrencyAmount: MoneyWith2DecimalPlaces{1},
			OriginCurrency:        "USD",
			OriginCurrencyAmount:  MoneyWith2DecimalPlaces{1},
			Amounts: map[string]AmountInCurrency{
				"USD": {Currency: "USD", Amount: MoneyWith2DecimalPlaces{1}},
				"AMD": {Currency: "AMD", Amount: MoneyWith2DecimalPlaces{400}},
			},
		},
		{
			IsExpense:             true,
			Date:                  date1,
			SourceType:            "t1",
			Source:                "s1",
			Details:               "expense, account USD, paid in AMD",
			Category:              "a",
			FromAccount:           "a1",
			ToAccount:             "a2",
			AccountCurrency:       "USD",
			AccountCurrencyAmount: MoneyWith2DecimalPlaces{2},
			OriginCurrency:        "AMD",
			OriginCurrencyAmount:  MoneyWith2DecimalPlaces{800},
			Amounts: map[string]AmountInCurrency{
				"USD": {Currency: "USD", Amount: MoneyWith2DecimalPlaces{2}},
				"AMD": {Currency: "AMD", Amount: MoneyWith2DecimalPlaces{800}},
			},
		},
		{
			IsExpense:             true,
			Date:                  date1,
			SourceType:            "t1",
			Source:                "s1",
			Details:               "expense, account AMD, paid in USD",
			Category:              "a",
			FromAccount:           "a1",
			ToAccount:             "a2",
			AccountCurrency:       "AMD",
			AccountCurrencyAmount: MoneyWith2DecimalPlaces{1200},
			OriginCurrency:        "USD",
			OriginCurrencyAmount:  MoneyWith2DecimalPlaces{3},
			Amounts: map[string]AmountInCurrency{
				"USD": {Currency: "USD", Amount: MoneyWith2DecimalPlaces{3}},
				"AMD": {Currency: "AMD", Amount: MoneyWith2DecimalPlaces{1200}},
			},
		},
		{
			IsExpense:             true,
			Date:                  date1,
			SourceType:            "t1",
			Source:                "s1",
			Details:               "expense, account AMD",
			Category:              "a",
			FromAccount:           "a1",
			ToAccount:             "a2",
			AccountCurrency:       "AMD",
			AccountCurrencyAmount: MoneyWith2DecimalPlaces{1600},
			Amounts: map[string]AmountInCurrency{
				"USD": {Currency: "USD", Amount: MoneyWith2DecimalPlaces{4}},
				"AMD": {Currency: "AMD", Amount: MoneyWith2DecimalPlaces{1600}},
			},
		},
		{
			IsExpense:             false,
			Date:                  date1,
			SourceType:            "t1",
			Source:                "s1",
			Details:               "income, account is AMD, paid in USD",
			Category:              "a",
			FromAccount:           "a1",
			ToAccount:             "a2",
			AccountCurrency:       "AMD",
			AccountCurrencyAmount: MoneyWith2DecimalPlaces{2000},
			OriginCurrency:        "USD",
			OriginCurrencyAmount:  MoneyWith2DecimalPlaces{5},
			Amounts: map[string]AmountInCurrency{
				"USD": {Currency: "USD", Amount: MoneyWith2DecimalPlaces{5}},
				"AMD": {Currency: "AMD", Amount: MoneyWith2DecimalPlaces{2000}},
			},
		},
	}
	expectedVariousCurrenciesFull := `AMD amounts:
Statistics for        2024-10-27..2024-10-28 (in    AMD):
  Income  (total  1 groups, filtered sum          20.00):
    a                                  :        20.00, from 1 transaction(s):
      2024-10-27	Income	       20.00 AMD	a	a1->a2	t1	s1	'income, account is AMD, paid in USD'	20.00 AMD (0)	0.05 USD (0)
  Expenses (total  1 groups, filt-ed sum          40.00):
    a                                  :        40.00, from 4 transaction(s):
      2024-10-27	Expense	        0.01 USD	a	a1->a2	t1	s1	'expense, all in USD'	4.00 AMD (0)	0.01 USD (0)
      2024-10-27	Expense	        0.02 USD	a	a1->a2	t1	s1	'expense, account USD, paid in AMD'	8.00 AMD (0)	0.02 USD (0)
      2024-10-27	Expense	       12.00 AMD	a	a1->a2	t1	s1	'expense, account AMD, paid in USD'	12.00 AMD (0)	0.03 USD (0)
      2024-10-27	Expense	       16.00 AMD	a	a1->a2	t1	s1	'expense, account AMD'	16.00 AMD (0)	0.04 USD (0)
USD amounts:
Statistics for        2024-10-27..2024-10-28 (in    USD):
  Income  (total  1 groups, filtered sum           0.05):
    a                                  :         0.05, from 1 transaction(s):
      2024-10-27	Income	       20.00 AMD	a	a1->a2	t1	s1	'income, account is AMD, paid in USD'	20.00 AMD (0)	0.05 USD (0)
  Expenses (total  1 groups, filt-ed sum           0.10):
    a                                  :         0.10, from 4 transaction(s):
      2024-10-27	Expense	        0.01 USD	a	a1->a2	t1	s1	'expense, all in USD'	4.00 AMD (0)	0.01 USD (0)
      2024-10-27	Expense	        0.02 USD	a	a1->a2	t1	s1	'expense, account USD, paid in AMD'	8.00 AMD (0)	0.02 USD (0)
      2024-10-27	Expense	       12.00 AMD	a	a1->a2	t1	s1	'expense, account AMD, paid in USD'	12.00 AMD (0)	0.03 USD (0)
      2024-10-27	Expense	       16.00 AMD	a	a1->a2	t1	s1	'expense, account AMD'	16.00 AMD (0)	0.04 USD (0)
`
	tests := []struct {
		name           string
		accounts       map[string]*AccountFromTransactions
		journalEntries []JournalEntry
		expected       string
	}{
		{
			name:           "various_groups_no_accounts",
			accounts:       map[string]*AccountFromTransactions{},
			journalEntries: jesVariousCategories,
			expected:       expectedVariousCategoriesFull,
		},
		{
			name:           "various_groups_not_mine_accounts",
			accounts:       map[string]*AccountFromTransactions{"a1": a1NotMine, "a2": a2NotMine, "a3": a3NotMine},
			journalEntries: jesVariousCategories,
			expected:       expectedVariousCategoriesFull,
		},
		{
			name:           "various_groups_a1_mine_account",
			accounts:       map[string]*AccountFromTransactions{"a1": a1Mine},
			journalEntries: jesVariousCategories,
			expected:       expectedVariousCategoriesA1Mine,
		},
		{
			name:           "various_currencies",
			accounts:       map[string]*AccountFromTransactions{"a1": a1NotMine, "a2": a2NotMine, "a3": a3NotMine},
			journalEntries: jesVariousCurrencies,
			expected: expectedVariousCurrenciesFull,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Arrange
			factoryMethod, _ := NewStatisticBuilderByCategories(tt.accounts)
			builder := factoryMethod(date1, date2)

			// Act
			for _, je := range tt.journalEntries {
				if err := builder.HandleJournalEntry(je, date1, date2); err != nil {
					t.Errorf("HandleJournalEntry() failed on %+v with %#v", je, err)
				}
			}

			// Assert
			actual := strings.Builder{}
			DumpIntervalStatistics(builder.GetIntervalStatistics(), &actual, "", true)
			assertStringEqual(t, actual.String(), tt.expected)
		})
	}
}

func assertStringEqual(t *testing.T, actual, expected string) {

	// Compare actual vs expected strings line by line
	actualLines := strings.Split(actual, "\n")
	expectedLines := strings.Split(expected, "\n")

	if len(actualLines) != len(expectedLines) {
		t.Errorf("Output has different number of lines - got %d, expected %d\n",
			len(actualLines), len(expectedLines))
	}
	linesCount := len(actualLines)
	if len(expectedLines) < linesCount {
		linesCount = len(expectedLines)
	}

	for i := 0; i < linesCount; i++ {
		if actualLines[i] != expectedLines[i] {
			// Find first differing character
			minLen := len(actualLines[i])
			if len(expectedLines[i]) < minLen {
				minLen = len(expectedLines[i])
			}

			diffPos := 0
			for diffPos < minLen && actualLines[i][diffPos] == expectedLines[i][diffPos] {
				diffPos++
			}

			t.Errorf("Line %d differs at position %d:\nExpected: %s\n  Actual: %s\n",
				i+1, diffPos,
				expectedLines[i],
				actualLines[i])
		}
	}
}

func newUsdJE(amount int, isExpense bool, category, from, to string) JournalEntry {
	sign := "+"
	if isExpense {
		sign = "-"
	}
	return JournalEntry{
		IsExpense:             isExpense,
		Date:                  date1,
		Category:              category,
		Details:               fmt.Sprintf("%s%d%s", sign, amount, category),
		AccountCurrencyAmount: MoneyWith2DecimalPlaces{amount},
		FromAccount:           from,
		ToAccount:             to,
		Amounts:               map[string]AmountInCurrency{"USD": {Currency: "USD", Amount: MoneyWith2DecimalPlaces{amount}}},
	}
}
