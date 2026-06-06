package currency

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/AlexanderMakarov/am-budget-view/internal/categorization"
	"github.com/AlexanderMakarov/am-budget-view/internal/config"
	"github.com/AlexanderMakarov/am-budget-view/internal/model"
)

var testDate = time.Now()

func TestParseExchangeRateFromDetails(t *testing.T) {
	source := &model.TransactionsSource{TypeName: "TestParseExchangeRateFromDetails", FilePath: "TestParseExchangeRateFromDetails"}
	tests := []struct {
		name            string
		date            time.Time
		details         string
		targetCurrency1 string
		targetCurrency2 string
		expected        *ExchangeRate
	}{
		{
			name:            "just-numbers",
			date:            testDate,
			details:         "330000 AMD / 4.4 = 75000 RUB",
			targetCurrency1: "AMD",
			targetCurrency2: "RUB",
			expected: &ExchangeRate{
				date:         testDate,
				currencyFrom: "AMD",
				currencyTo:   "RUB",
				exchangeRate: 4.4,
				source:       source,
			},
		},
		{
			name:            "commas-and-dots",
			date:            testDate,
			details:         "330,000.00 AMD / 4.4 = 75,000.00 RUB",
			targetCurrency1: "AMD",
			targetCurrency2: "RUB",
			expected: &ExchangeRate{
				date:         testDate,
				currencyFrom: "AMD",
				currencyTo:   "RUB",
				exchangeRate: 4.4,
				source:       source,
			},
		},
		{
			name:            "reverse-params",
			date:            testDate,
			details:         "330,000.00 AMD / 4.4 = 75,000.00 RUB",
			targetCurrency1: "RUB",
			targetCurrency2: "AMD",
			expected: &ExchangeRate{
				date:         testDate,
				currencyFrom: "RUB",
				currencyTo:   "AMD",
				exchangeRate: 0.22727272727272727,
				source:       source,
			},
		},
		{
			name:            "wrong-currencies",
			date:            testDate,
			details:         "330,000.00 USD / 4.4 = 75,000.00 RUB",
			targetCurrency1: "AMD",
			targetCurrency2: "",
			expected:        nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := parseExchangeRateFromDetails(test.date, test.details, test.targetCurrency1, test.targetCurrency2, source)
			if test.expected == nil {
				if actual != nil {
					t.Errorf("Expected nil, got %+v", actual)
				}
			} else {
				if actual == nil {
					t.Errorf("Expected %+v, got nil", test.expected)
				} else if !reflect.DeepEqual(test.expected, actual) {
					t.Errorf("Expected %+v, got %+v", test.expected, actual)
				}
			}
		})
	}
}

func TestFindClosestExchangeRateToCurrency(t *testing.T) {
	rates := []*ExchangeRate{
		{date: testDate.AddDate(0, 0, -2), currencyFrom: "AMD", currencyTo: "RUB", exchangeRate: 4.7},
		{date: testDate.AddDate(0, 0, -1), currencyFrom: "AMD", currencyTo: "RUB", exchangeRate: 4.6},
		{date: testDate, currencyFrom: "AMD", currencyTo: "RUB", exchangeRate: 4.5},
		{date: testDate.AddDate(0, 0, 1), currencyFrom: "AMD", currencyTo: "RUB", exchangeRate: 4.4},
		{date: testDate.AddDate(0, 0, 2), currencyFrom: "AMD", currencyTo: "RUB", exchangeRate: 4.3},
	}
	tests := []struct {
		name                 string
		date                 time.Time
		targetCurrency       string
		curState             *currencyState
		expectedExchangeRate *ExchangeRate
		expectedDays         int
	}{
		{
			name:           "1no-rates",
			date:           testDate,
			targetCurrency: "AMD",
			curState: &currencyState{
				currency: "AMD",
				statistics: &CurrencyStatistics{
					ExchangeRates: []*ExchangeRate{},
				},
				exchangeRateIndexesPerCurrency: map[string]int{},
			},
			expectedExchangeRate: nil,
			expectedDays:         math.MaxInt,
		},
		{
			name:           "2another-currency",
			date:           testDate,
			targetCurrency: "USD",
			curState: &currencyState{
				currency: "AMD",
				statistics: &CurrencyStatistics{
					ExchangeRates: rates,
				},
				exchangeRateIndexesPerCurrency: map[string]int{},
			},
			expectedExchangeRate: nil,
			expectedDays:         math.MaxInt,
		},
		{
			name:           "3same-date-rate_not-init",
			date:           testDate,
			targetCurrency: "AMD",
			curState: &currencyState{
				currency: "AMD",
				statistics: &CurrencyStatistics{
					ExchangeRates: rates[2:],
				},
				exchangeRateIndexesPerCurrency: map[string]int{},
			},
			expectedExchangeRate: rates[2],
			expectedDays:         0,
		},
		{
			name:           "4before-rates_not-init",
			date:           testDate,
			targetCurrency: "AMD",
			curState: &currencyState{
				currency: "AMD",
				statistics: &CurrencyStatistics{
					ExchangeRates: rates[0:2],
				},
				exchangeRateIndexesPerCurrency: map[string]int{},
			},
			expectedExchangeRate: rates[1],
			expectedDays:         1,
		},
		{
			name:           "5all-rates_not-init",
			date:           testDate,
			targetCurrency: "AMD",
			curState: &currencyState{
				currency: "AMD",
				statistics: &CurrencyStatistics{
					ExchangeRates: rates,
				},
				exchangeRateIndexesPerCurrency: map[string]int{},
			},
			expectedExchangeRate: rates[2],
			expectedDays:         0,
		},
		{
			name:           "6all-rates_init-before",
			date:           testDate,
			targetCurrency: "AMD",
			curState: &currencyState{
				currency: "AMD",
				statistics: &CurrencyStatistics{
					ExchangeRates: rates,
				},
				exchangeRateIndexesPerCurrency: map[string]int{
					"AMD": 0,
				},
			},
			expectedExchangeRate: rates[2],
			expectedDays:         0,
		},
		{
			name:           "7all-rates_init-same-day",
			date:           testDate,
			targetCurrency: "AMD",
			curState: &currencyState{
				currency: "AMD",
				statistics: &CurrencyStatistics{
					ExchangeRates: rates,
				},
				exchangeRateIndexesPerCurrency: map[string]int{
					"AMD": 2,
				},
			},
			expectedExchangeRate: rates[2],
			expectedDays:         0,
		},
		{
			name:           "8all-rates_init-after",
			date:           testDate,
			targetCurrency: "AMD",
			curState: &currencyState{
				currency: "AMD",
				statistics: &CurrencyStatistics{
					ExchangeRates: rates,
				},
				exchangeRateIndexesPerCurrency: map[string]int{
					"AMD": 3,
				},
			},
			expectedExchangeRate: rates[3],
			expectedDays:         1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualExchangeRate, actualDays := findClosestExchangeRateToCurrency(test.date, test.targetCurrency, test.curState)
			if actualExchangeRate != test.expectedExchangeRate {
				expectedStr := "nil"
				if test.expectedExchangeRate != nil {
					expectedStr = fmt.Sprintf("&{\n  date:%s\n  currencyFrom:%s\n  currencyTo:%s\n  exchangeRate:%f\n  source:%s\n}",
						test.expectedExchangeRate.date.Format("2006-01-02 15:04:05"),
						test.expectedExchangeRate.currencyFrom,
						test.expectedExchangeRate.currencyTo,
						test.expectedExchangeRate.exchangeRate,
						test.expectedExchangeRate.source)
				}
				actualStr := "nil"
				if actualExchangeRate != nil {
					actualStr = fmt.Sprintf("&{\n  date:%s\n  currencyFrom:%s\n  currencyTo:%s\n  exchangeRate:%f\n  source:%s\n}",
						actualExchangeRate.date.Format("2006-01-02 15:04:05"),
						actualExchangeRate.currencyFrom,
						actualExchangeRate.currencyTo,
						actualExchangeRate.exchangeRate,
						actualExchangeRate.source)
				}
				t.Errorf("Expected rate %s, got %s", expectedStr, actualStr)
			}
			if actualDays != test.expectedDays {
				t.Errorf("Expected days %d, got %d", test.expectedDays, actualDays)
			}
		})
	}
}

func TestBuildDataMart_ConstantExchangeRates(t *testing.T) {
	// Arrange
	// Create test transactions with no exchange rates.
	transactions := []model.Transaction{
		{
			Date:            testDate,
			AccountCurrency: "USD",
			Amount:          model.MoneyWith2DecimalPlaces{Cents: 10000}, // $100.00
			Details:         "check USD",
			FromAccount:     "Assets:Bank:USD",
			ToAccount:       "Expenses:Test",
			IsExpense:       true,
			Source:          &model.TransactionsSource{TypeName: "Test", FilePath: "test.csv"},
		},
		{
			Date:            testDate.AddDate(0, 0, 1),
			AccountCurrency: "AMD",
			Amount:          model.MoneyWith2DecimalPlaces{Cents: 3810000}, // 38,100 AMD
			Details:         "check AMD",
			FromAccount:     "Assets:Bank:AMD",
			ToAccount:       "Expenses:Test",
			IsExpense:       true,
			Source:          &model.TransactionsSource{TypeName: "Test", FilePath: "test.csv"},
		},
		{
			Date:            testDate.AddDate(0, 0, 1),
			AccountCurrency: "AMD",
			Amount:          model.MoneyWith2DecimalPlaces{Cents: 100}, // 1 AMD
			Details:         "small amount",
			FromAccount:     "Assets:Bank:AMD",
			ToAccount:       "Expenses:Test",
			IsExpense:       true,
			Source:          &model.TransactionsSource{TypeName: "Test", FilePath: "test.csv"},
		},
	}
	// Create config with constant exchange rates.
	cfg := &config.Config{
		ExchangeRates: map[string]map[string]float64{
			"USD": {
				"AMD": 381,
				"EUR": 0.9,
			},
		},
		ConvertToCurrencies: []string{"USD", "AMD"},
		Groups: map[string]*config.GroupConfig{
			"Test": {
				Substrings: []string{"Test"},
			},
		},
	}

	// Act
	dataMart, err := BuildDataMart(transactions, cfg)

	// Assert
	if err != nil {
		t.Fatalf("BuildDataMart failed: %v", err)
	}
	// Check that USD currency was added.
	usdCurrency := dataMart.AllCurrencies["USD"]
	if usdCurrency == nil {
		t.Fatal("USD currency was not generated")
	}
	// Check that 2 USD ExchangeRates were generated - for USD -> AMD and USD -> EUR.
	if len(usdCurrency.ExchangeRates) != 2 {
		t.Fatalf("USD currency: expected 2 exchange rates, got %+v", usdCurrency.ExchangeRates)
	}
	// Compare list of expected exchange rates with list of actual exchange rates order-independent.
	errMsg := assertExchangeRates(
		[]*ExchangeRate{
			{date: testDate, currencyFrom: "USD", currencyTo: "AMD", exchangeRate: 381, source: &model.TransactionsSource{TypeName: model.ConstantExchangeRateSourceName, FilePath: model.ConstantExchangeRateSourceFilePath}},
			{date: testDate, currencyFrom: "USD", currencyTo: "EUR", exchangeRate: 0.9, source: &model.TransactionsSource{TypeName: model.ConstantExchangeRateSourceName, FilePath: model.ConstantExchangeRateSourceFilePath}},
		},
		usdCurrency.ExchangeRates,
	)
	if errMsg != nil {
		t.Fatalf("USD currency: %s", *errMsg)
	}
	// Check that AMD currency was added.
	amdCurrency := dataMart.AllCurrencies["AMD"]
	if amdCurrency == nil {
		t.Fatal("AMD currency was not generated")
	}
	// Check that 2 AMD ExchangeRates were generated - for USD -> AMD and USD -> EUR.
	if len(amdCurrency.ExchangeRates) != 1 {
		t.Fatalf("AMD currency: expected 1 exchange rate, got %+v", amdCurrency.ExchangeRates)
	}
	// Compare list of expected exchange rates with list of actual exchange rates order-independent.
	errMsg = assertExchangeRates(
		[]*ExchangeRate{
			{date: testDate, currencyFrom: "AMD", currencyTo: "USD", exchangeRate: 1.0 / 381, source: &model.TransactionsSource{TypeName: model.ConstantExchangeRateSourceName, FilePath: model.ConstantExchangeRateSourceFilePath}},
		},
		amdCurrency.ExchangeRates,
	)
	if errMsg != nil {
		t.Fatalf("AMD currency: %s", *errMsg)
	}
}

func TestConvertToCurrency(t *testing.T) {
	tests := []struct {
		name              string
		amount            model.MoneyWith2DecimalPlaces
		amountCurrency    string
		targetCurrency    string
		date              time.Time
		curStates         map[string]*currencyState
		expectedAmount    model.MoneyWith2DecimalPlaces
		expectedPrecision int
		expectedPath      []string
	}{
		{
			name:              "same currency",
			amount:            model.MoneyWith2DecimalPlaces{Cents: 100},
			amountCurrency:    "AMD",
			targetCurrency:    "AMD",
			date:              testDate,
			curStates:         map[string]*currencyState{},
			expectedAmount:    model.MoneyWith2DecimalPlaces{Cents: 100},
			expectedPrecision: 0,
			expectedPath:      []string{},
		},
		{
			name:           "direct",
			amount:         model.MoneyWith2DecimalPlaces{Cents: 100000}, // $1000.00
			amountCurrency: "USD",
			targetCurrency: "AMD",
			date:           testDate,
			curStates: map[string]*currencyState{
				"USD": {
					currency: "USD",
					statistics: &CurrencyStatistics{
						ExchangeRates: []*ExchangeRate{
							{
								date:         testDate,
								currencyFrom: "USD",
								currencyTo:   "AMD",
								exchangeRate: 1.0 / 381,
								source:       &model.TransactionsSource{TypeName: "test", FilePath: "test.csv"},
							},
						},
					},
					exchangeRateIndexesPerCurrency: map[string]int{
						"USD": 0,
					},
				},
			},
			expectedAmount:    model.MoneyWith2DecimalPlaces{Cents: 38100000},
			expectedPrecision: 1, // Same day conversion.
			expectedPath:      []string{buildConversionPath("USD", "AMD", 1.0/381, testDate, &model.TransactionsSource{TypeName: "test", FilePath: "test.csv"})},
		},
		{
			name:           "direct via constant exchange rate",
			amount:         model.MoneyWith2DecimalPlaces{Cents: 100000}, // 1000.00 AMD
			amountCurrency: "AMD",
			targetCurrency: "USD",
			date:           testDate,
			curStates: map[string]*currencyState{
				"AMD": {
					currency: "AMD",
					statistics: &CurrencyStatistics{
						ExchangeRates: []*ExchangeRate{
							{
								date:         testDate,
								currencyFrom: "AMD",
								currencyTo:   "USD",
								exchangeRate: 381,
								source:       &model.TransactionsSource{TypeName: model.ConstantExchangeRateSourceName, FilePath: model.ConstantExchangeRateSourceFilePath},
							},
						},
					},
					exchangeRateIndexesPerCurrency: map[string]int{
						"USD": 0,
					},
				},
			},
			expectedAmount:    model.MoneyWith2DecimalPlaces{Cents: 262},
			expectedPrecision: 100500, // Constant exchange rate precision.
			expectedPath:      []string{buildConversionPath("AMD", "USD", 381, testDate, &model.TransactionsSource{TypeName: model.ConstantExchangeRateSourceName, FilePath: model.ConstantExchangeRateSourceFilePath})},
		},
		{
			name:           "conversion of very small amount",
			amount:         model.MoneyWith2DecimalPlaces{Cents: 100}, // 1.00 AMD
			amountCurrency: "AMD",
			targetCurrency: "USD",
			date:           testDate,
			curStates: map[string]*currencyState{
				"AMD": {
					currency: "AMD",
					statistics: &CurrencyStatistics{
						ExchangeRates: []*ExchangeRate{
							{
								date:         testDate,
								currencyFrom: "AMD",
								currencyTo:   "USD",
								exchangeRate: 381,
								source:       &model.TransactionsSource{TypeName: "test", FilePath: "test.csv"}},
						},
					},
					exchangeRateIndexesPerCurrency: map[string]int{
						"USD": 0,
					},
				},
			},
			expectedAmount:    model.MoneyWith2DecimalPlaces{Cents: 1}, // Expecting 0.01 USD in spite of 1 / 381 = 0.0026 USD
			expectedPrecision: 1,
			expectedPath:      []string{buildConversionPath("AMD", "USD", 381, testDate, &model.TransactionsSource{TypeName: "test", FilePath: "test.csv"})},
		},
		{
			name:           "multiple conversions",
			amount:         model.MoneyWith2DecimalPlaces{Cents: 100000}, // 1000.00 AMD
			amountCurrency: "AMD",                                // AMD -> USD -> EUR
			targetCurrency: "EUR",
			date:           testDate,
			curStates: map[string]*currencyState{
				"AMD": {
					currency: "AMD",
					statistics: &CurrencyStatistics{
						ExchangeRates: []*ExchangeRate{
							{date: testDate, currencyFrom: "AMD", currencyTo: "USD", exchangeRate: 381, source: &model.TransactionsSource{TypeName: "test", FilePath: "test.csv"}},
						},
					},
					exchangeRateIndexesPerCurrency: map[string]int{
						"AMD": 0,
						"USD": 0,
						"EUR": 0,
					},
				},
				"USD": {
					currency: "USD",
					statistics: &CurrencyStatistics{
						ExchangeRates: []*ExchangeRate{
							{date: testDate, currencyFrom: "USD", currencyTo: "EUR", exchangeRate: 1.0 / 0.9, source: &model.TransactionsSource{TypeName: "test", FilePath: "test.csv"}},
						},
					},
					exchangeRateIndexesPerCurrency: map[string]int{
						"AMD": 0,
						"USD": 0,
						"EUR": 0,
					},
				},
				"EUR": {
					currency: "EUR",
					statistics: &CurrencyStatistics{
						ExchangeRates: []*ExchangeRate{
							{date: testDate, currencyFrom: "EUR", currencyTo: "USD", exchangeRate: 0.9, source: &model.TransactionsSource{TypeName: "test", FilePath: "test.csv"}},
						},
					},
					exchangeRateIndexesPerCurrency: map[string]int{
						"AMD": 0,
						"USD": 0,
						"EUR": 0,
					},
				},
			},
			// 1000 / 381 * 0.9 = 2.36 EUR but after precision losses it is 2.35.
			expectedAmount: model.MoneyWith2DecimalPlaces{Cents: 235},
			// Precision is 1 day to USD conversion + 1 day to EUR conversion.
			expectedPrecision: 2,
			expectedPath:      []string{buildConversionPath("AMD", "USD", 381, testDate, &model.TransactionsSource{TypeName: "test", FilePath: "test.csv"}), buildConversionPath("USD", "EUR", 1.0/0.9, testDate, &model.TransactionsSource{TypeName: "test", FilePath: "test.csv"})},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualAmount, actualPrecision, actualPath := convertToCurrency(test.amount, test.amountCurrency, test.targetCurrency, test.date, test.curStates)
			if actualAmount != test.expectedAmount {
				t.Errorf("Expected amount %+v, got %+v", test.expectedAmount, actualAmount)
			}
			if actualPrecision != test.expectedPrecision {
				t.Errorf("Expected precision %d, got %d", test.expectedPrecision, actualPrecision)
			}
			if !reflect.DeepEqual(actualPath, test.expectedPath) {
				t.Errorf("Expected path %+v, got %+v", test.expectedPath, actualPath)
			}
		})
	}
}

func issue13Config() *config.Config {
	return &config.Config{
		ExchangeRates: map[string]map[string]float64{
			"USD": {"AMD": 380, "EUR": 0.86, "RUB": 80},
		},
		ConvertToCurrencies: []string{"AMD", "USD", "RUB"},
		Groups: map[string]*config.GroupConfig{
			"expense": {Substrings: []string{"expense", "payment"}},
		},
	}
}

func issue13Transactions(includeRUB bool) []model.Transaction {
	source := &model.TransactionsSource{TypeName: "Ameria", FilePath: "history.csv"}
	transactions := []model.Transaction{
		{
			Date:            time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC),
			AccountCurrency: "AMD",
			Amount:          model.MoneyWith2DecimalPlaces{Cents: 100000},
			Details:         "AMD expense",
			FromAccount:     "acc-amd",
			ToAccount:       "exp1",
			IsExpense:       true,
			Source:          source,
		},
		{
			Date:            time.Date(2025, 10, 2, 0, 0, 0, 0, time.UTC),
			AccountCurrency: "USD",
			Amount:          model.MoneyWith2DecimalPlaces{Cents: 10000},
			Details:         "USD expense",
			FromAccount:     "acc-usd",
			ToAccount:       "exp1",
			IsExpense:       true,
			Source:          source,
		},
	}
	if includeRUB {
		transactions = append(transactions, model.Transaction{
			Date:            time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC),
			AccountCurrency: "RUB",
			Amount:          model.MoneyWith2DecimalPlaces{Cents: 800000},
			Details:         "RUB expense",
			FromAccount:     "acc-rub",
			ToAccount:       "exp1",
			IsExpense:       true,
			Source:          source,
		})
	}
	return transactions
}

// TestBuildDataMart_Issue13 documents how config and transaction inputs populate
// AllCurrencies vs ConvertibleCurrencies (https://github.com/AlexanderMakarov/am-budget-view/issues/13).
func TestBuildDataMart_Issue13(t *testing.T) {
	tests := []struct {
		name                       string
		includeRUBInTransactions   bool
		wantAllCurrencies          []string
		wantConvertibleCurrencies  []string
		wantRUBInAllCurrencies     bool
		wantRUBInConvertible       bool
	}{
		{
			// Config lists RUB as a conversion target; transactions only have AMD/USD rows
			// with no embedded exchange-rate pairs. RUB is config-only: it lands in
			// ConvertibleCurrencies (with fallback rates) but not in AllCurrencies.
			name:                      "config lists RUB, transactions have AMD and USD only",
			includeRUBInTransactions:  false,
			wantAllCurrencies:         []string{"AMD", "USD"},
			wantConvertibleCurrencies: []string{"AMD", "USD", "RUB"},
			wantRUBInAllCurrencies:    false,
			wantRUBInConvertible:      true,
		},
		{
			// Same config, but a transaction row uses AccountCurrency=RUB.
			// RUB is collected from transactions and appears in both maps.
			name:                      "config lists RUB, transactions include RUB account rows",
			includeRUBInTransactions:  true,
			wantAllCurrencies:         []string{"AMD", "USD", "RUB"},
			wantConvertibleCurrencies: []string{"AMD", "USD", "RUB"},
			wantRUBInAllCurrencies:    true,
			wantRUBInConvertible:      true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dataMart, err := BuildDataMart(issue13Transactions(test.includeRUBInTransactions), issue13Config())
			if err != nil {
				t.Fatalf("BuildDataMart: %v", err)
			}

			assertCurrencyNames(t, "AllCurrencies", dataMart.AllCurrencies, test.wantAllCurrencies)
			assertCurrencyNames(t, "ConvertibleCurrencies", dataMart.ConvertibleCurrencies, test.wantConvertibleCurrencies)

			_, rubInAll := dataMart.AllCurrencies["RUB"]
			_, rubInConvertible := dataMart.ConvertibleCurrencies["RUB"]
			if rubInAll != test.wantRUBInAllCurrencies {
				t.Fatalf("RUB in AllCurrencies = %v, want %v", rubInAll, test.wantRUBInAllCurrencies)
			}
			if rubInConvertible != test.wantRUBInConvertible {
				t.Fatalf("RUB in ConvertibleCurrencies = %v, want %v", rubInConvertible, test.wantRUBInConvertible)
			}
		})
	}
}

// TestBuildJournalEntries_Issue13 documents outcomes for config + transaction input pairs.
func TestBuildJournalEntries_Issue13(t *testing.T) {
	tests := []struct {
		name                    string
		transactions            []model.Transaction
		cfg                     *config.Config
		removeFromAllCurrencies []string
		wantPanic               bool
		wantError               bool
	}{
		{
			// convertToCurrencies includes config-only RUB; transactions are AMD/USD only.
			// Converting AMD/USD into RUB fails with an error, not a panic.
			name:                    "config-only RUB target, AMD and USD transactions",
			transactions:            issue13Transactions(false),
			cfg:                     issue13Config(),
			removeFromAllCurrencies: nil,
			wantPanic:               false,
			wantError:               true,
		},
		{
			// RUB appears on a transaction row, so BuildDataMart keeps RUB in AllCurrencies.
			name:                    "RUB in config and on transaction rows",
			transactions:            issue13Transactions(true),
			cfg:                     issue13Config(),
			removeFromAllCurrencies: nil,
			wantPanic:               false,
			wantError:               false,
		},
		{
			// Panic condition: a row has AccountCurrency=RUB (convert FROM RUB), but RUB is
			// absent from AllCurrencies/curStates. BuildDataMart normally prevents this;
			// convertToCurrency must not dereference a missing curState.
			name: "RUB transaction row with RUB missing from AllCurrencies",
			transactions: []model.Transaction{{
				Date:            time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC),
				AccountCurrency: "RUB",
				Amount:          model.MoneyWith2DecimalPlaces{Cents: 800000},
				Details:         "RUB expense",
				FromAccount:     "acc-rub",
				ToAccount:       "exp1",
				IsExpense:       true,
				Source:          &model.TransactionsSource{TypeName: "Ameria", FilePath: "history.csv"},
			}},
			cfg: func() *config.Config {
				cfg := issue13Config()
				cfg.ConvertToCurrencies = []string{"AMD", "USD"}
				return cfg
			}(),
			removeFromAllCurrencies: []string{"RUB"},
			wantPanic:               true,
			wantError:               false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cat, err := categorization.NewCategorization(test.cfg)
			if err != nil {
				t.Fatalf("NewCategorization: %v", err)
			}

			dataMart, err := BuildDataMart(test.transactions, test.cfg)
			if err != nil {
				t.Fatalf("BuildDataMart: %v", err)
			}
			for _, currency := range test.removeFromAllCurrencies {
				delete(dataMart.AllCurrencies, currency)
			}

			var panicValue any
			func() {
				defer func() {
					panicValue = recover()
				}()
				_, _, err = BuildJournalEntries(dataMart, cat)
			}()

			if test.wantPanic {
				if panicValue == nil {
					t.Fatal("expected panic when converting from a currency missing from AllCurrencies")
				}
				return
			}
			if panicValue != nil {
				t.Fatalf("unexpected panic: %v", panicValue)
			}
			if test.wantError && err == nil {
				t.Fatal("expected conversion error, got nil")
			}
			if !test.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestConvertToCurrency_Issue13 is the minimal reproduction of the nil dereference:
// convert FROM a currency that is not present in curStates.
func TestConvertToCurrency_Issue13(t *testing.T) {
	rateDate := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when amountCurrency is missing from curStates")
		}
	}()

	convertToCurrency(
		model.MoneyWith2DecimalPlaces{Cents: 800000},
		"RUB",
		"AMD",
		rateDate,
		map[string]*currencyState{
			"USD": {
				currency: "USD",
				statistics: &CurrencyStatistics{
					ExchangeRates: []*ExchangeRate{
						{date: rateDate, currencyFrom: "USD", currencyTo: "AMD", exchangeRate: 1.0 / 380},
					},
				},
				exchangeRateIndexesPerCurrency: map[string]int{},
			},
		},
	)
}

func assertCurrencyNames(t *testing.T, mapName string, currencies map[string]*CurrencyStatistics, want []string) {
	t.Helper()
	got := make([]string, 0, len(currencies))
	for name := range currencies {
		got = append(got, name)
	}
	slices.Sort(got)
	wantSorted := append([]string(nil), want...)
	slices.Sort(wantSorted)
	if !reflect.DeepEqual(got, wantSorted) {
		t.Fatalf("%s currencies = %v, want %v", mapName, got, wantSorted)
	}
}

func assertExchangeRates(expectedExchangeRates []*ExchangeRate, actualExchangeRates []*ExchangeRate) *string {
	for _, expectedExchangeRate := range expectedExchangeRates {
		found := false
		for _, actualExchangeRate := range actualExchangeRates {
			if expectedExchangeRate.currencyFrom == actualExchangeRate.currencyFrom && expectedExchangeRate.currencyTo == actualExchangeRate.currencyTo {
				found = true
				break
			}
		}
		if !found {
			errMsg := fmt.Sprintf("expected exchange rate %+v not found in actual exchange rates %+v", expectedExchangeRate, actualExchangeRates)
			return &errMsg
		}
	}
	return nil
}
