package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestInecoExcelFileParserParseRawTransactionsFromFile(t *testing.T) {
	validFileRegularPath := filepath.Join("testdata", "ineco", "valid_regular.xlsx")
	validFileCardPath := filepath.Join("testdata", "ineco", "valid_card.xlsx")
	sourceRegular := &TransactionsSource{
		TypeName:        "Inecobank XLSX statement",
		Tag:             "InecoExcelRegular:AMD",
		FilePath:        validFileRegularPath,
		AccountNumber:   "2050205020502050",
		AccountCurrency: "AMD",
	}
	sourceCard := &TransactionsSource{
		TypeName:        "Inecobank XLSX statement",
		Tag:             "InecoExcelCard:AMD",
		FilePath:        validFileCardPath,
		AccountNumber:   "1234567890121234",
		AccountCurrency: "AMD",
	}

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
					Source:               sourceRegular,
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
					Source:               sourceRegular,
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
					Source:               sourceCard,
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
					Source:               sourceCard,
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
					Source:               sourceCard,
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
			testName := tt.fileName + ".xlsx"
			filePath := filepath.Join("testdata", "ineco", testName)
			actual, err := parser.ParseRawTransactionsFromFile(filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("error=%+v, wantErr=%+v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.expectedResult, actual, moneyComparer, diffOnlyTransformer); diff != "" {
				t.Errorf("transaction %s mismatch (-expected +got):\n%s", testName, diff)
			}
		})
	}
}
