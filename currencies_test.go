package main

import (
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
				exchangeRate: 0.22727272727272727,
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
				exchangeRate: 0.22727272727272727,
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
				exchangeRate: 4.4,
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
			actual := parseExchangeRateFromDetails(test.date, test.details, test.targetCurrency1, test.targetCurrency2)
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

func TestFindClosestExchangeRate(t *testing.T) {
	rates := []*ExchangeRate{
		{date: testDate.AddDate(0, 0, -2), currencyFrom: "AMD", currencyTo: "RUB", exchangeRate: 4.7},
		{date: testDate.AddDate(0, 0, -1), currencyFrom: "AMD", currencyTo: "RUB", exchangeRate: 4.6},
		{date: testDate, currencyFrom: "AMD", currencyTo: "RUB", exchangeRate: 4.5},
		{date: testDate.AddDate(0, 0, 1), currencyFrom: "AMD", currencyTo: "RUB", exchangeRate: 4.4},
		{date: testDate.AddDate(0, 0, 2), currencyFrom: "AMD", currencyTo: "RUB", exchangeRate: 4.3},
	}
	tests := []struct {
		name          string
		date          time.Time
		curState      *currencyState
		expectedDays  int
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
			name: "same-date-rate-nolast",
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
			actualDays := findClosestExchangeRate(test.date, test.curState)
			// Assert expected days
			if actualDays != test.expectedDays {
				t.Errorf("Expected %d, got %d", test.expectedDays, actualDays)
			}
		})
	}
}
