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
					IsExpense:   true,
					Date:        time.Date(2024, time.June, 3, 0, 0, 0, 0, time.UTC),
					Details:     "Միջբանկային փոխանցում",
					Amount:      MoneyWith2DecimalPlaces{int: 10000000},
					Source:      validFileRegularPath,
					Currency:    "AMD",
					FromAccount: "Regular:205020502050-2050",
					ToAccount:   "",
				},
				{
					IsExpense:   false,
					Date:        time.Date(2024, time.June, 7, 0, 0, 0, 0, time.UTC),
					Details:     "Փոխանցում իմ հաշիվների միջև, Account replenishment, InecoOnline, 07/06/2023 11:38:58",
					Amount:      MoneyWith2DecimalPlaces{int: 7800010},
					Source:      validFileRegularPath,
					Currency:    "AMD",
					FromAccount: "",
					ToAccount:   "Regular:205020502050-2050",
				},
			},
		},
		{
			fileName:      "valid_card",
			detailsIncome: []string{},
			wantErr:       false,
			expectedResult: []Transaction{
				{
					IsExpense:   true,
					Date:        time.Date(2024, time.June, 5, 0, 0, 0, 0, time.UTC),
					Details:     "Անկանխիկ գործարք - WILDBERRIES - YEREVAN",
					Amount:      MoneyWith2DecimalPlaces{int: 10010},
					Source:      validFileCardPath,
					Currency:    "AMD",
					FromAccount: "Card:123456789012-1234",
					ToAccount:   "",
				},
				{
					IsExpense:   true,
					Date:        time.Date(2024, time.June, 1, 0, 0, 0, 0, time.UTC),
					Details:     "Անկանխիկ գործարք – CLOUD",
					Amount:      MoneyWith2DecimalPlaces{int: 784},
					Source:      validFileCardPath,
					Currency:    "USD",
					FromAccount: "Card:123456789012-1234",
					ToAccount:   "",
				},
				{
					IsExpense:   false,
					Date:        time.Date(2024, time.June, 7, 0, 0, 0, 0, time.UTC),
					Details:     "Գումարի ետ վերադարձ քարտապանին",
					Amount:      MoneyWith2DecimalPlaces{int: 99999999999},
					Source:      validFileCardPath,
					Currency:    "AMD",
					FromAccount: "",
					ToAccount:   "Card:123456789012-1234",
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
