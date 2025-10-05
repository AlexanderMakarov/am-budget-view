package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestMyAmeriaExcelFileParserParseRawTransactionsFromFile(t *testing.T) {
	validFilePath := filepath.Join("testdata", "ameria", "valid_file.xls")
	source := &TransactionsSource{
		TypeName:        "MyAmeria History XLS",
		Tag:             "MyAmeriaXls:AMD",
		FilePath:        validFilePath,
		AccountNumber:   "1234567890123456",
		AccountCurrency: "AMD",
	}
	tests := []struct {
		name               string
		filePath           string
		myAccounts         map[string]string
		detailsIncome      []string
		wantErr            bool
		expectedResult     []Transaction
		expectedSourceType string
	}{
		{
			name:          "valid_file-check_by_account",
			filePath:      validFilePath,
			myAccounts:    map[string]string{"1234567890123456": "AMD"},
			detailsIncome: []string{},
			wantErr:       false,
			expectedResult: []Transaction{
				{
					IsExpense:            true,
					Date:                 time.Date(2024, time.April, 20, 0, 0, 0, 0, time.UTC),
					Details:              "ԱԱՀ այդ թվում` 16.67%",
					Source:               source,
					AccountCurrency:      "AMD",
					Amount:               MoneyWith2DecimalPlaces{int: 10010},
					OriginCurrency:       "",
					OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 0},
					FromAccount:          "1234567890123456",
					ToAccount:            "9999999999999999",
				},
				{
					IsExpense:            true,
					Date:                 time.Date(2024, time.April, 21, 0, 0, 0, 0, time.UTC),
					Details:              "Payment for services",
					Source:               source,
					AccountCurrency:      "AMD",
					Amount:               MoneyWith2DecimalPlaces{int: 0},
					OriginCurrency:       "USD",
					OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 50010},
					FromAccount:          "1234567890123456",
					ToAccount:            "9999999999999999",
				},
				{
					IsExpense:            false,
					Date:                 time.Date(2024, time.April, 22, 0, 0, 0, 0, time.UTC),
					Details:              "Transfer to myself",
					Source:               source,
					AccountCurrency:      "AMD",
					Amount:               MoneyWith2DecimalPlaces{int: 0},
					OriginCurrency:       "USD",
					OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 100000},
					FromAccount:          "9999999999999999",
					ToAccount:            "1234567890123456",
				},
				{
					IsExpense:            false,
					Date:                 time.Date(2024, time.April, 19, 0, 0, 0, 0, time.UTC),
					Details:              "Բանկի ձևանմուշից տարբերվող տեղեկա",
					Source:               source,
					AccountCurrency:      "AMD",
					Amount:               MoneyWith2DecimalPlaces{int: 99999999999},
					OriginCurrency:       "",
					OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 0},
					FromAccount:          "9999999999999999",
					ToAccount:            "1234567890123456",
				},
			},
			expectedSourceType: "MyAmeriaExcel",
		},
		{
			name:               "file_not_found",
			filePath:           filepath.Join("testdata", "ameria", "non_existent_file.xls"),
			myAccounts:         map[string]string{},
			detailsIncome:      []string{},
			wantErr:            true,
			expectedResult:     nil,
			expectedSourceType: "",
		},
		{
			name:               "invalid_header",
			filePath:           filepath.Join("testdata", "ameria", "invalid_header.xls"),
			myAccounts:         map[string]string{},
			detailsIncome:      []string{},
			wantErr:            true,
			expectedResult:     nil,
			expectedSourceType: "",
		},
		{
			name:               "no_data",
			filePath:           filepath.Join("testdata", "ameria", "no_data.xls"),
			myAccounts:         map[string]string{},
			detailsIncome:      []string{},
			wantErr:            false,
			expectedResult:     []Transaction{},
			expectedSourceType: "MyAmeriaExcel",
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
				return
			}
			if err == nil {
				if diff := cmp.Diff(tt.expectedResult, actual, moneyComparer, diffOnlyTransformer); diff != "" {
					t.Errorf("ParseRawTransactionsFromFile() mismatch (-expected +got):\n%s", diff)
				}
			}
		})
	}
}
