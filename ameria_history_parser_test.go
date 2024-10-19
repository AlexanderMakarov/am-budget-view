package main

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestMyAmeriaExcelFileParserParseRawTransactionsFromFile(t *testing.T) {
	validFilePath := filepath.Join("testdata", "ameria", "valid_file.xls")
	tests := []struct {
		name           string
		filePath       string
		myAccounts     []string
		detailsIncome  []string
		wantErr        bool
		expectedResult []Transaction
	}{
		{
			name:          "valid_file-check_by_account",
			filePath:      validFilePath,
			myAccounts:    []string{"1234567890123456"},
			detailsIncome: []string{},
			wantErr:       false,
			expectedResult: []Transaction{
				{
					IsExpense:            true,
					Date:                 time.Date(2024, time.April, 20, 0, 0, 0, 0, time.UTC),
					Details:              "ԱԱՀ այդ թվում` 16.67%",
					SourceType:           "MyAmeriaExcel:AMD",
					Source:               validFilePath,
					AccountCurrency:      "",
					OriginCurrency:       "AMD",
					OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 10010},
					FromAccount:          "1234567890123456",
					ToAccount:            "9999999999999999",
				},
				{
					IsExpense:            true,
					Date:                 time.Date(2024, time.April, 21, 0, 0, 0, 0, time.UTC),
					Details:              "Payment for services",
					SourceType:           "MyAmeriaExcel:USD",
					Source:               validFilePath,
					AccountCurrency:      "",
					OriginCurrency:       "USD",
					OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 50010},
					FromAccount:          "1234567890123456",
					ToAccount:            "9999999999999999",
				},
				{
					IsExpense:            false,
					Date:                 time.Date(2024, time.April, 22, 0, 0, 0, 0, time.UTC),
					Details:              "Transfer to myself",
					SourceType:           "MyAmeriaExcel:USD",
					Source:               validFilePath,
					AccountCurrency:      "",
					OriginCurrency:       "USD",
					OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 100000},
					FromAccount:          "9999999999999999",
					ToAccount:            "1234567890123456",
				},
				{
					IsExpense:            false,
					Date:                 time.Date(2024, time.April, 19, 0, 0, 0, 0, time.UTC),
					Details:              "Բանկի ձևանմուշից տարբերվող տեղեկա",
					SourceType:           "MyAmeriaExcel:AMD",
					Source:               validFilePath,
					AccountCurrency:      "",
					OriginCurrency:       "AMD",
					OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 99999999999},
					FromAccount:          "9999999999999999",
					ToAccount:            "1234567890123456",
				},
			},
		},
		{
			name:           "file_not_found",
			filePath:       filepath.Join("testdata", "ameria", "non_existent_file.xls"),
			myAccounts:     []string{},
			detailsIncome:  []string{},
			wantErr:        true,
			expectedResult: nil,
		},
		{
			name:           "invalid_header",
			filePath:       filepath.Join("testdata", "ameria", "invalid_header.xls"),
			myAccounts:     []string{},
			detailsIncome:  []string{},
			wantErr:        true,
			expectedResult: nil,
		},
		{
			name:           "no_data",
			filePath:       filepath.Join("testdata", "ameria", "no_data.xls"),
			myAccounts:     []string{},
			detailsIncome:  []string{},
			wantErr:        false,
			expectedResult: []Transaction{},
		},
		//add tests with "(AMD)" columns
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := MyAmeriaExcelFileParser{
				MyAccounts: tt.myAccounts,
			}
			actual, err := parser.ParseRawTransactionsFromFile(tt.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRawTransactionsFromFile() error = %+v, wantErr %+v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(actual, tt.expectedResult) {
				t.Errorf("ParseRawTransactionsFromFile() actual=%+v, expected=%+v", actual, tt.expectedResult)
			}
		})
	}
}
