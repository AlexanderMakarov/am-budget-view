package main

import (
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"
)

var testDate = time.Now()

func TestParseExchangeRateFromDetails(t *testing.T) {
	source := &TransactionsSource{TypeName: "TestParseExchangeRateFromDetails", FilePath: "TestParseExchangeRateFromDetails"}
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
	transactions := []Transaction{
		{
			Date:            testDate,
			AccountCurrency: "USD",
			Amount:          MoneyWith2DecimalPlaces{int: 10000}, // $100.00
			Details:         "check USD",
			FromAccount:     "Assets:Bank:USD",
			ToAccount:       "Expenses:Test",
			IsExpense:       true,
			Source:          &TransactionsSource{TypeName: "Test", FilePath: "test.csv"},
		},
		{
			Date:            testDate.AddDate(0, 0, 1),
			AccountCurrency: "AMD",
			Amount:          MoneyWith2DecimalPlaces{int: 3810000}, // 38,100 AMD
			Details:         "check AMD",
			FromAccount:     "Assets:Bank:AMD",
			ToAccount:       "Expenses:Test",
			IsExpense:       true,
			Source:          &TransactionsSource{TypeName: "Test", FilePath: "test.csv"},
		},
		{
			Date:            testDate.AddDate(0, 0, 1),
			AccountCurrency: "AMD",
			Amount:          MoneyWith2DecimalPlaces{int: 100}, // 1 AMD
			Details:         "small amount",
			FromAccount:     "Assets:Bank:AMD",
			ToAccount:       "Expenses:Test",
			IsExpense:       true,
			Source:          &TransactionsSource{TypeName: "Test", FilePath: "test.csv"},
		},
	}
	// Create config with constant exchange rates.
	config := &Config{
		ExchangeRates: map[string]map[string]float64{
			"USD": {
				"AMD": 381,
				"EUR": 0.9,
			},
		},
		ConvertToCurrencies: []string{"USD", "AMD"},
		Groups: map[string]*GroupConfig{
			"Test": {
				Substrings: []string{"Test"},
			},
		},
	}

	// Act
	dataMart, err := BuildDataMart(transactions, config)

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
			{date: testDate, currencyFrom: "USD", currencyTo: "AMD", exchangeRate: 381, source: &TransactionsSource{TypeName: ConstantExchangeRateSourceName, FilePath: ConstantExchangeRateSourceFilePath}},
			{date: testDate, currencyFrom: "USD", currencyTo: "EUR", exchangeRate: 0.9, source: &TransactionsSource{TypeName: ConstantExchangeRateSourceName, FilePath: ConstantExchangeRateSourceFilePath}},
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
			{date: testDate, currencyFrom: "AMD", currencyTo: "USD", exchangeRate: 1.0 / 381, source: &TransactionsSource{TypeName: ConstantExchangeRateSourceName, FilePath: ConstantExchangeRateSourceFilePath}},
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
		amount            MoneyWith2DecimalPlaces
		amountCurrency    string
		targetCurrency    string
		date              time.Time
		curStates         map[string]*currencyState
		expectedAmount    MoneyWith2DecimalPlaces
		expectedPrecision int
		expectedPath      []string
	}{
		{
			name:              "same currency",
			amount:            MoneyWith2DecimalPlaces{int: 100},
			amountCurrency:    "AMD",
			targetCurrency:    "AMD",
			date:              testDate,
			curStates:         map[string]*currencyState{},
			expectedAmount:    MoneyWith2DecimalPlaces{int: 100},
			expectedPrecision: 0,
			expectedPath:      []string{},
		},
		{
			name:           "direct",
			amount:         MoneyWith2DecimalPlaces{int: 100000}, // $1000.00
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
								source:       &TransactionsSource{TypeName: "test", FilePath: "test.csv"},
							},
						},
					},
					exchangeRateIndexesPerCurrency: map[string]int{
						"USD": 0,
					},
				},
			},
			expectedAmount:    MoneyWith2DecimalPlaces{int: 38100000},
			expectedPrecision: 1, // Same day conversion.
			expectedPath:      []string{buildConversionPath("USD", "AMD", 1.0/381, testDate, &TransactionsSource{TypeName: "test", FilePath: "test.csv"})},
		},
		{
			name:           "direct via constant exchange rate",
			amount:         MoneyWith2DecimalPlaces{int: 100000}, // 1000.00 AMD
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
								source:       &TransactionsSource{TypeName: ConstantExchangeRateSourceName, FilePath: ConstantExchangeRateSourceFilePath},
							},
						},
					},
					exchangeRateIndexesPerCurrency: map[string]int{
						"USD": 0,
					},
				},
			},
			expectedAmount:    MoneyWith2DecimalPlaces{int: 262},
			expectedPrecision: 100500, // Constant exchange rate precision.
			expectedPath:      []string{buildConversionPath("AMD", "USD", 381, testDate, &TransactionsSource{TypeName: ConstantExchangeRateSourceName, FilePath: ConstantExchangeRateSourceFilePath})},
		},
		{
			name:           "conversion of very small amount",
			amount:         MoneyWith2DecimalPlaces{int: 100}, // 1.00 AMD
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
								source:       &TransactionsSource{TypeName: "test", FilePath: "test.csv"}},
						},
					},
					exchangeRateIndexesPerCurrency: map[string]int{
						"USD": 0,
					},
				},
			},
			expectedAmount:    MoneyWith2DecimalPlaces{int: 1}, // Expecting 0.01 USD in spite of 1 / 381 = 0.0026 USD
			expectedPrecision: 1,
			expectedPath:      []string{buildConversionPath("AMD", "USD", 381, testDate, &TransactionsSource{TypeName: "test", FilePath: "test.csv"})},
		},
		{
			name:           "multiple conversions",
			amount:         MoneyWith2DecimalPlaces{int: 100000}, // 1000.00 AMD
			amountCurrency: "AMD",                                // AMD -> USD -> EUR
			targetCurrency: "EUR",
			date:           testDate,
			curStates: map[string]*currencyState{
				"AMD": {
					currency: "AMD",
					statistics: &CurrencyStatistics{
						ExchangeRates: []*ExchangeRate{
							{date: testDate, currencyFrom: "AMD", currencyTo: "USD", exchangeRate: 381, source: &TransactionsSource{TypeName: "test", FilePath: "test.csv"}},
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
							{date: testDate, currencyFrom: "USD", currencyTo: "EUR", exchangeRate: 1.0 / 0.9, source: &TransactionsSource{TypeName: "test", FilePath: "test.csv"}},
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
							{date: testDate, currencyFrom: "EUR", currencyTo: "USD", exchangeRate: 0.9, source: &TransactionsSource{TypeName: "test", FilePath: "test.csv"}},
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
			expectedAmount: MoneyWith2DecimalPlaces{int: 235},
			// Precision is 1 day to USD conversion + 1 day to EUR conversion.
			expectedPrecision: 2,
			expectedPath:      []string{buildConversionPath("AMD", "USD", 381, testDate, &TransactionsSource{TypeName: "test", FilePath: "test.csv"}), buildConversionPath("USD", "EUR", 1.0/0.9, testDate, &TransactionsSource{TypeName: "test", FilePath: "test.csv"})},
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
