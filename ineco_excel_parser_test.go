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
		name                  string
		fileName              string
		detailsIncome         []string
		wantErr               bool
		expectedResult        []Transaction
		expectedSourceType    string
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
					SourceType:           "InecoExcelRegular:AMD",
					Source:               validFileRegularPath,
					AccountCurrency:      "AMD",
					Amount:               MoneyWith2DecimalPlaces{int: 400000},
					OriginCurrency:       "",
					OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 400000},
					FromAccount:          "2050205020502050",
					ToAccount:            "UnknownAccount",
				},
				{
					IsExpense:            false,
					Date:                 time.Date(2024, time.June, 7, 0, 0, 0, 0, time.UTC),
					Details:              "Փոխանցում իմ հաշիվների միջև, Account replenishment, InecoOnline, 07/06/2023 11:38:58",
					SourceType:           "InecoExcelRegular:AMD",
					Source:               validFileRegularPath,
					AccountCurrency:      "AMD",
					Amount:               MoneyWith2DecimalPlaces{int: 7800010},
					OriginCurrency:       "",
					OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 7800010},
					FromAccount:          "UnknownAccount",
					ToAccount:            "2050205020502050",
				},
			},
			expectedSourceType: "InecoExcelRegular:AMD",
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
					SourceType:           "InecoExcelCard:AMD",
					Source:               validFileCardPath,
					AccountCurrency:      "AMD",
					Amount:               MoneyWith2DecimalPlaces{int: 35000},
					OriginCurrency:       "",
					OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 35000},
					FromAccount:          "1234567890121234",
					ToAccount:            "UnknownAccount",
				},
				{
					IsExpense:            true,
					Date:                 time.Date(2024, time.June, 1, 0, 0, 0, 0, time.UTC),
					Details:              "Անկանխիկ գործարք – CLOUD",
					SourceType:           "InecoExcelCard:AMD",
					Source:               validFileCardPath,
					AccountCurrency:      "AMD",
					Amount:               MoneyWith2DecimalPlaces{int: 784},
					OriginCurrency:       "USD",
					OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 2},
					FromAccount:          "1234567890121234",
					ToAccount:            "UnknownAccount",
				},
				{
					IsExpense:            false,
					Date:                 time.Date(2024, time.June, 7, 0, 0, 0, 0, time.UTC),
					Details:              "Գումարի ետ վերադարձ քարտապանին",
					SourceType:           "InecoExcelCard:AMD",
					Source:               validFileCardPath,
					AccountCurrency:      "AMD",
					Amount:               MoneyWith2DecimalPlaces{int: 99999999999},
					OriginCurrency:       "",
					OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 99999999999},
					FromAccount:          "UnknownAccount",
					ToAccount:            "1234567890121234",
				},
			},
			expectedSourceType: "InecoExcelCard:AMD",
		},
		{
			fileName:       "non_existent",
			detailsIncome:  []string{},
			wantErr:        true,
			expectedResult:       nil,
			expectedSourceType:   "",
		},
		{
			fileName:       "invalid_header",
			detailsIncome:  []string{},
			wantErr:        true,
			expectedResult: nil,
			expectedSourceType: "",
		},
		{
			fileName:       "no_account_number_label",
			detailsIncome:  []string{},
			wantErr:        true,
			expectedResult:       nil,
			expectedSourceType:   "",
		},
		{
			fileName:       "no_data",
			detailsIncome:  []string{},
			wantErr:        false,
			expectedResult: []Transaction{},
			expectedSourceType: "InecoExcelCard:AMD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.fileName, func(t *testing.T) {
			parser := InecoExcelFileParser{}
			filePath := filepath.Join("testdata", "ineco", tt.fileName+".xlsx")
			actual, returnedSourceType, err := parser.ParseRawTransactionsFromFile(filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("error=%+v, wantErr=%+v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(actual, tt.expectedResult) {
				t.Errorf("actual=%+v, expected=%+v", actual, tt.expectedResult)
			}
			if returnedSourceType != tt.expectedSourceType {
				t.Errorf("ParseRawTransactionsFromFile() returned source type = %+v, expected=%+v", returnedSourceType, tt.expectedSourceType)
			}
		})
	}
}
