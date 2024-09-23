package main

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestInecoExcelFileParserParseRawTransactionsFromFile(t *testing.T) {
	validFileRegularPath := filepath.Join("testdata", "ineco", "valid_regular.xlsx")
	validFileCardPath := filepath.Join("testdata", "ineco", "valid_card.xlsx")

	tests := []struct {
		name           string
		fileName       string
		detailsIncome  []string
		wantErr        bool
		expectedResult []Transaction
	}{
		{
			fileName:      "valid_regular",
			detailsIncome: []string{},
			wantErr:       false,
			expectedResult: []Transaction{
				{
					IsExpense:            true,
					Date:                 time.Date(2024, time.June, 3, 0, 0, 0, 0, time.UTC),
					Details:              "Միջբանկային փոխանցում",
					SourceType:           "InecoExcelAMD",
					Source:               validFileRegularPath,
					AccountCurrency:      "AMD",
					Amount:               MoneyWith2DecimalPlaces{int: 400000},
					OriginCurrency:       "AMD",
					OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 400000},
					FromAccount:          "Regular:205020502050-2050",
					ToAccount:            "UnknownAccount",
				},
				{
					IsExpense:            false,
					Date:                 time.Date(2024, time.June, 7, 0, 0, 0, 0, time.UTC),
					Details:              "Փոխանցում իմ հաշիվների միջև, Account replenishment, InecoOnline, 07/06/2023 11:38:58",
					SourceType:           "InecoExcelAMD",
					Source:               validFileRegularPath,
					AccountCurrency:      "AMD",
					Amount:               MoneyWith2DecimalPlaces{int: 7800010},
					OriginCurrency:       "AMD",
					OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 7800010},
					FromAccount:          "UnknownAccount",
					ToAccount:            "Regular:205020502050-2050",
				},
			},
		},
		{
			fileName:      "valid_card",
			detailsIncome: []string{},
			wantErr:       false,
			expectedResult: []Transaction{
				{
					IsExpense:            true,
					Date:                 time.Date(2024, time.June, 5, 0, 0, 0, 0, time.UTC),
					Details:              "Անկանխիկ գործարք - WILDBERRIES - YEREVAN",
					SourceType:           "InecoExcelAMD",
					Source:               validFileCardPath,
					AccountCurrency:      "AMD",
					Amount:               MoneyWith2DecimalPlaces{int: 35000},
					OriginCurrency:       "AMD",
					OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 35000},
					FromAccount:          "Card:123456789012-1234",
					ToAccount:            "UnknownAccount",
				},
				{
					IsExpense:            true,
					Date:                 time.Date(2024, time.June, 1, 0, 0, 0, 0, time.UTC),
					Details:              "Անկանխիկ գործարք – CLOUD",
					SourceType:           "InecoExcelAMD",
					Source:               validFileCardPath,
					AccountCurrency:      "AMD",
					Amount:               MoneyWith2DecimalPlaces{int: 784},
					OriginCurrency:       "USD",
					OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 2},
					FromAccount:          "Card:123456789012-1234",
					ToAccount:            "UnknownAccount",
				},
				{
					IsExpense:            false,
					Date:                 time.Date(2024, time.June, 7, 0, 0, 0, 0, time.UTC),
					Details:              "Գումարի ետ վերադարձ քարտապանին",
					SourceType:           "InecoExcelAMD",
					Source:               validFileCardPath,
					AccountCurrency:      "AMD",
					Amount:               MoneyWith2DecimalPlaces{int: 99999999999},
					OriginCurrency:       "AMD",
					OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 99999999999},
					FromAccount:          "UnknownAccount",
					ToAccount:            "Card:123456789012-1234",
				},
			},
		},
		{
			fileName:       "non_existent",
			detailsIncome:  []string{},
			wantErr:        true,
			expectedResult: nil,
		},
		{
			fileName:       "invalid_header",
			detailsIncome:  []string{},
			wantErr:        true,
			expectedResult: nil,
		},
		{
			fileName:       "no_account_number_label",
			detailsIncome:  []string{},
			wantErr:        true,
			expectedResult: nil,
		},
		{
			fileName:       "no_data",
			detailsIncome:  []string{},
			wantErr:        false,
			expectedResult: []Transaction{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.fileName, func(t *testing.T) {
			parser := InecoExcelFileParser{}
			filePath := filepath.Join("testdata", "ineco", tt.fileName+".xlsx")
			actual, err := parser.ParseRawTransactionsFromFile(filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("error=%+v, wantErr=%+v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(actual, tt.expectedResult) {
				t.Errorf("actual=%+v, expected=%+v", actual, tt.expectedResult)
			}
		})
	}
}
