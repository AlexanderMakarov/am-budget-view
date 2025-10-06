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
		accounts           map[string]*AccountStatistics
		config             *Config
		expectedMyAccounts map[string]struct{}
	}{
		{
			"no_accounts",
			map[string]*AccountStatistics{},
			nil,
			map[string]struct{}{},
		},
		{
			"no_my_accounts",
			map[string]*AccountStatistics{
				"a": {Number: "a", IsTransactionAccount: false},
			},
			nil,
			map[string]struct{}{},
		},
		{
			"many_my_accounts",
			map[string]*AccountStatistics{
				"a": {Number: "a", IsTransactionAccount: true},
				"b": {Number: "b", IsTransactionAccount: false},
				"c": {Number: "c", IsTransactionAccount: true},
			},
			nil,
			map[string]struct{}{"a": {}, "c": {}},
		},
		{
			"config_my_accounts_only",
			map[string]*AccountStatistics{
				"x": {Number: "x", IsTransactionAccount: false},
			},
			&Config{MyAccounts: []string{"m1", "m2"}},
			map[string]struct{}{"m1": {}, "m2": {}},
		},
	}
	const testName = "NewGroupExtractorByCategories()"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Act
			builder, err := NewStatisticBuilderByCategories(tt.accounts, tt.config)
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
	source := &TransactionsSource{TypeName: "t1", FilePath: "s1"}
	a1Mine := &AccountStatistics{Number: "a1", IsTransactionAccount: true}
	a1NotMine := &AccountStatistics{Number: "a1", IsTransactionAccount: false}
	a2NotMine := &AccountStatistics{Number: "a2", IsTransactionAccount: false}
	a3NotMine := &AccountStatistics{Number: "a3", IsTransactionAccount: false}
	jesVariousCategories := []JournalEntry{
		newUsdJE(1, false, "a", "a1", "a2", source),
		newUsdJE(2, false, "a", "a3", "a1", source),
		newUsdJE(3, true, "b", "a1", "a3", source),
		newUsdJE(4, true, "b", "a2", "a3", source),
		newUsdJE(5, true, "b", "a2", "a3", source),
		newUsdJE(6, false, "b", "a1", "a2", source),
		newUsdJE(7, false, "c", "a2", "a3", source),
		newUsdJE(8, true, "c", "a1", "a3", source),
		newUsdJE(9, false, "c", "a3", "a1", source),
		newUsdJE(10, true, "c", "a2", "a1", source),
		newUsdJE(11, false, "c", "a3", "a1", source),
		newUsdJE(12, false, "e", "a2", "a1", source),
		newUsdJE(13, true, "e", "a3", "a2", source),
	}
	expectedVariousCategoriesFull := `USD amounts:
Statistics for        2024-10-27..2024-10-28 (in    USD):
  Income  (total  4 groups, filtered sum           0.48):
    c                                    :         0.27, from 3 transaction(s):
      2024-10-27	Income	        0.07 	c	a2->a3	t1	s1	'+7c'	0.07 USD (0)
      2024-10-27	Income	        0.09 	c	a3->a1	t1	s1	'+9c'	0.09 USD (0)
      2024-10-27	Income	        0.11 	c	a3->a1	t1	s1	'+11c'	0.11 USD (0)
    e                                    :         0.12, from 1 transaction(s):
      2024-10-27	Income	        0.12 	e	a2->a1	t1	s1	'+12e'	0.12 USD (0)
    b                                    :         0.06, from 1 transaction(s):
      2024-10-27	Income	        0.06 	b	a1->a2	t1	s1	'+6b'	0.06 USD (0)
    a                                    :         0.03, from 2 transaction(s):
      2024-10-27	Income	        0.01 	a	a1->a2	t1	s1	'+1a'	0.01 USD (0)
      2024-10-27	Income	        0.02 	a	a3->a1	t1	s1	'+2a'	0.02 USD (0)
  Expenses (total  3 groups, filt-ed sum           0.43):
    c                                    :         0.18, from 2 transaction(s):
      2024-10-27	Expense	        0.08 	c	a1->a3	t1	s1	'-8c'	0.08 USD (0)
      2024-10-27	Expense	        0.10 	c	a2->a1	t1	s1	'-10c'	0.10 USD (0)
    e                                    :         0.13, from 1 transaction(s):
      2024-10-27	Expense	        0.13 	e	a3->a2	t1	s1	'-13e'	0.13 USD (0)
    b                                    :         0.12, from 3 transaction(s):
      2024-10-27	Expense	        0.03 	b	a1->a3	t1	s1	'-3b'	0.03 USD (0)
      2024-10-27	Expense	        0.04 	b	a2->a3	t1	s1	'-4b'	0.04 USD (0)
      2024-10-27	Expense	        0.05 	b	a2->a3	t1	s1	'-5b'	0.05 USD (0)
`
	expectedVariousCategoriesA1Mine := `USD amounts:
Statistics for        2024-10-27..2024-10-28 (in    USD):
  Income  (total  3 groups, filtered sum           0.41):
    c                                    :         0.27, from 3 transaction(s):
      2024-10-27	Income	        0.07 	c	a2->a3	t1	s1	'+7c'	0.07 USD (0)
      2024-10-27	Income	        0.09 	c	a3->a1	t1	s1	'+9c'	0.09 USD (0)
      2024-10-27	Income	        0.11 	c	a3->a1	t1	s1	'+11c'	0.11 USD (0)
    e                                    :         0.12, from 1 transaction(s):
      2024-10-27	Income	        0.12 	e	a2->a1	t1	s1	'+12e'	0.12 USD (0)
    a                                    :         0.02, from 2 transaction(s):
      2024-10-27	Income	        0.01 	a	a1->a2	t1	s1	'+1a'	0.01 USD (0)
      2024-10-27	Income	        0.02 	a	a3->a1	t1	s1	'+2a'	0.02 USD (0)
  Expenses (total  3 groups, filt-ed sum           0.33):
    e                                    :         0.13, from 1 transaction(s):
      2024-10-27	Expense	        0.13 	e	a3->a2	t1	s1	'-13e'	0.13 USD (0)
    b                                    :         0.12, from 3 transaction(s):
      2024-10-27	Expense	        0.03 	b	a1->a3	t1	s1	'-3b'	0.03 USD (0)
      2024-10-27	Expense	        0.04 	b	a2->a3	t1	s1	'-4b'	0.04 USD (0)
      2024-10-27	Expense	        0.05 	b	a2->a3	t1	s1	'-5b'	0.05 USD (0)
    c                                    :         0.08, from 2 transaction(s):
      2024-10-27	Expense	        0.08 	c	a1->a3	t1	s1	'-8c'	0.08 USD (0)
      2024-10-27	Expense	        0.10 	c	a2->a1	t1	s1	'-10c'	0.10 USD (0)
`
	jesVariousCurrencies := []JournalEntry{
		{
			IsExpense:             true,
			Date:                  date1,
			Source:                source,
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
			Source:                source,
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
			Source:                source,
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
			Source:                source,
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
			Source:                source,
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
    a                                    :        20.00, from 1 transaction(s):
      2024-10-27	Income	       20.00 AMD	a	a1->a2	t1	s1	'income, account is AMD, paid in USD'	20.00 AMD (0)	0.05 USD (0)
  Expenses (total  1 groups, filt-ed sum          40.00):
    a                                    :        40.00, from 4 transaction(s):
      2024-10-27	Expense	        0.01 USD	a	a1->a2	t1	s1	'expense, all in USD'	4.00 AMD (0)	0.01 USD (0)
      2024-10-27	Expense	        0.02 USD	a	a1->a2	t1	s1	'expense, account USD, paid in AMD'	8.00 AMD (0)	0.02 USD (0)
      2024-10-27	Expense	       12.00 AMD	a	a1->a2	t1	s1	'expense, account AMD, paid in USD'	12.00 AMD (0)	0.03 USD (0)
      2024-10-27	Expense	       16.00 AMD	a	a1->a2	t1	s1	'expense, account AMD'	16.00 AMD (0)	0.04 USD (0)
USD amounts:
Statistics for        2024-10-27..2024-10-28 (in    USD):
  Income  (total  1 groups, filtered sum           0.05):
    a                                    :         0.05, from 1 transaction(s):
      2024-10-27	Income	       20.00 AMD	a	a1->a2	t1	s1	'income, account is AMD, paid in USD'	20.00 AMD (0)	0.05 USD (0)
  Expenses (total  1 groups, filt-ed sum           0.10):
    a                                    :         0.10, from 4 transaction(s):
      2024-10-27	Expense	        0.01 USD	a	a1->a2	t1	s1	'expense, all in USD'	4.00 AMD (0)	0.01 USD (0)
      2024-10-27	Expense	        0.02 USD	a	a1->a2	t1	s1	'expense, account USD, paid in AMD'	8.00 AMD (0)	0.02 USD (0)
      2024-10-27	Expense	       12.00 AMD	a	a1->a2	t1	s1	'expense, account AMD, paid in USD'	12.00 AMD (0)	0.03 USD (0)
      2024-10-27	Expense	       16.00 AMD	a	a1->a2	t1	s1	'expense, account AMD'	16.00 AMD (0)	0.04 USD (0)
`
	tests := []struct {
		name           string
		accounts       map[string]*AccountStatistics
		config         *Config
		journalEntries []JournalEntry
		expected       string
	}{
		{
			name:           "various_groups_no_accounts",
			accounts:       map[string]*AccountStatistics{},
			config:         nil,
			journalEntries: jesVariousCategories,
			expected:       expectedVariousCategoriesFull,
		},
		{
			name:           "various_groups_not_mine_accounts",
			accounts:       map[string]*AccountStatistics{"a1": a1NotMine, "a2": a2NotMine, "a3": a3NotMine},
			config:         nil,
			journalEntries: jesVariousCategories,
			expected:       expectedVariousCategoriesFull,
		},
		{
			name:           "various_groups_a1_mine_account",
			accounts:       map[string]*AccountStatistics{"a1": a1Mine},
			config:         nil,
			journalEntries: jesVariousCategories,
			expected:       expectedVariousCategoriesA1Mine,
		},
		{
			name:           "various_groups_config_a1_mine",
			accounts:       map[string]*AccountStatistics{"a1": a1NotMine, "a2": a2NotMine, "a3": a3NotMine},
			config:         &Config{MyAccounts: []string{"a1"}},
			journalEntries: jesVariousCategories,
			expected:       expectedVariousCategoriesA1Mine,
		},
		{
			name:           "various_currencies",
			accounts:       map[string]*AccountStatistics{"a1": a1NotMine, "a2": a2NotMine, "a3": a3NotMine},
			config:         nil,
			journalEntries: jesVariousCurrencies,
			expected:       expectedVariousCurrenciesFull,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Arrange
			factoryMethod, _ := NewStatisticBuilderByCategories(tt.accounts, tt.config)
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

func newUsdJE(amount int, isExpense bool, category, from, to string, source *TransactionsSource) JournalEntry {
	sign := "+"
	if isExpense {
		sign = "-"
	}
	return JournalEntry{
		IsExpense:             isExpense,
		Date:                  date1,
		Source:                source,
		Category:              category,
		Details:               fmt.Sprintf("%s%d%s", sign, amount, category),
		AccountCurrencyAmount: MoneyWith2DecimalPlaces{amount},
		FromAccount:           from,
		ToAccount:             to,
		Amounts:               map[string]AmountInCurrency{"USD": {Currency: "USD", Amount: MoneyWith2DecimalPlaces{amount}}},
	}
}
