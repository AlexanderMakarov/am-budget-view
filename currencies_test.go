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
				source:       "TestParseExchangeRateFromDetails",
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
				source:       "TestParseExchangeRateFromDetails",
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
				source:       "TestParseExchangeRateFromDetails",
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
			actual := parseExchangeRateFromDetails(test.date, test.details, test.targetCurrency1, test.targetCurrency2, "TestParseExchangeRateFromDetails")
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

func TestFindAnyCurrencyClosestExchangeRate(t *testing.T) {
	rates := []*ExchangeRate{
		{date: testDate.AddDate(0, 0, -2), currencyFrom: "AMD", currencyTo: "RUB", exchangeRate: 4.7},
		{date: testDate.AddDate(0, 0, -1), currencyFrom: "AMD", currencyTo: "RUB", exchangeRate: 4.6},
		{date: testDate, currencyFrom: "AMD", currencyTo: "RUB", exchangeRate: 4.5},
		{date: testDate.AddDate(0, 0, 1), currencyFrom: "AMD", currencyTo: "RUB", exchangeRate: 4.4},
		{date: testDate.AddDate(0, 0, 2), currencyFrom: "AMD", currencyTo: "RUB", exchangeRate: 4.3},
	}
	tests := []struct {
		name         string
		date         time.Time
		curState     *currencyState
		expectedDays int
	}{
		{
			name: "empty",
			date: testDate,
			curState: &currencyState{
				currency: "AMD",
				statistics: &CurrencyStatistics{
					ExchangeRates: []*ExchangeRate{},
				},
			},
			expectedDays: math.MaxInt,
		},
		{
			name: "same-date-ratenolast",
			date: testDate,
			curState: &currencyState{
				currency: "AMD",
				statistics: &CurrencyStatistics{
					ExchangeRates: rates[2:3],
				},
			},
			expectedDays: 0,
		},
		{
			name: "same-date-rate-lastset",
			date: testDate,
			curState: &currencyState{
				currency: "AMD",
				statistics: &CurrencyStatistics{
					ExchangeRates: rates[2:3],
				},
				exchangeRateIndexesPerCurrency: map[string]int{
					"AMD": 0,
				},
			},
			expectedDays: 0,
		},
		{
			name: "other-date-rate-nolast",
			date: testDate,
			curState: &currencyState{
				currency: "AMD",
				statistics: &CurrencyStatistics{
					ExchangeRates: rates[0:1],
				},
			},
			expectedDays: 2,
		},
		{
			name: "many-rates-nolast",
			date: testDate,
			curState: &currencyState{
				currency: "AMD",
				statistics: &CurrencyStatistics{
					ExchangeRates: rates,
				},
			},
			expectedDays: 0,
		},
		{
			name: "many-rates-lastbefore",
			date: testDate,
			curState: &currencyState{
				currency: "AMD",
				statistics: &CurrencyStatistics{
					ExchangeRates: rates,
				},
				exchangeRateIndexesPerCurrency: map[string]int{
					"AMD": 1,
				},
			},
			expectedDays: 0,
		},
		{
			name: "many-rates-lastsameday",
			date: testDate,
			curState: &currencyState{
				currency: "AMD",
				statistics: &CurrencyStatistics{
					ExchangeRates: rates,
				},
				exchangeRateIndexesPerCurrency: map[string]int{
					"AMD": 2,
				},
			},
			expectedDays: 0,
		},
		{
			name: "many-rates-lastafter",
			date: testDate,
			curState: &currencyState{
				currency: "AMD",
				statistics: &CurrencyStatistics{
					ExchangeRates: rates,
				},
				exchangeRateIndexesPerCurrency: map[string]int{
					"AMD": 4,
				},
			},
			expectedDays: 0,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualDays := findAnyCurrencyClosestExchangeRate(test.date, test.curState)
			// Assert expected days
			if actualDays != test.expectedDays {
				t.Errorf("Expected %d, got %d", test.expectedDays, actualDays)
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
