package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestAcbaRegularAccountExcelFileParser_ParseRawTransactionsFromFile_Valid(t *testing.T) {
	validFilePath := filepath.Join("testdata", "acba", "valid_account.xls")

	accountNumber := "1234567890123456"
	accountCurrency := "AMD"
	source := &TransactionsSource{
		TypeName:        "Acba Regular Account XLS statement",
		Tag:             "AcbaAccountExcel:" + accountCurrency,
		FilePath:        validFilePath,
		AccountNumber:   accountNumber,
		AccountCurrency: accountCurrency,
	}

	got, err := AcbaRegularAccountExcelFileParser{}.ParseRawTransactionsFromFile(validFilePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []Transaction{
		{
			Date:                 time.Date(2025, time.September, 27, 0, 0, 0, 0, time.UTC),
			IsExpense:            true,
			Amount:               MoneyWith2DecimalPlaces{int: 160},
			FromAccount:          accountNumber,
			ToAccount:            "220483381467000",
			Source:               source,
			AccountCurrency:      accountCurrency,
			OriginCurrency:       "",
			OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 0},
			Details:              "Փոխանցում հաշվին (մոբայլ բանկինգ 189909288) - հ/հ 220483381467000  ստացող Name Surname",
		},
		{
			Date:                 time.Date(2025, time.September, 30, 0, 0, 0, 0, time.UTC),
			IsExpense:            true,
			Amount:               MoneyWith2DecimalPlaces{int: 14490},
			FromAccount:          accountNumber,
			ToAccount:            "",
			Source:               source,
			AccountCurrency:      accountCurrency,
			OriginCurrency:       "",
			OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 0},
			Details:              "Խնայողական հաշվի եկամտահարկ",
		},
		{
			Date:                 time.Date(2025, time.September, 30, 0, 0, 0, 0, time.UTC),
			IsExpense:            false,
			Amount:               MoneyWith2DecimalPlaces{int: 144900},
			FromAccount:          "220485212843000",
			ToAccount:            accountNumber,
			Source:               source,
			AccountCurrency:      accountCurrency,
			OriginCurrency:       "",
			OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 0},
			Details:              "Խնայողական հաշվի տոկոսի վճարում - հ/հ 220485212843000  փոխանցող Name Surname",
		},
	}

	if diff := cmp.Diff(expected, got, moneyComparer, diffOnlyTransformer); diff != "" {
		t.Fatalf("transactions mismatch (-expected +got):\n%s", diff)
	}
}

func TestAcbaRegularAccountExcelFileParser_ParseRawTransactionsFromFile_NoData(t *testing.T) {
	validFilePath := filepath.Join("testdata", "acba", "no_transactions_and_finish_text_account.xls")
	transactions, err := AcbaRegularAccountExcelFileParser{}.ParseRawTransactionsFromFile(validFilePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	} else if len(transactions) != 0 {
		t.Errorf("expected 0 transactions, got %d", len(transactions))
	}
}

func TestAcbaRegularAccountXlsxFileParser_ParseRawTransactionsFromFile_Errors(t *testing.T) {
	tests := []struct {
		fileName     string
		errorMessage string
	}{
		{
			fileName:     "no_data_account",
			errorMessage: "after scanning 19 rows can't find headers 'ԱմսաթիվԳումարԱրժույթՄուտքԵլք'",
		},
		{
			fileName:     "no_account_number_account",
			errorMessage: "can't find account number and/or currency down to row 13",
		},
		{
			fileName:     "no_account_currency_account",
			errorMessage: "can't find account number and/or currency down to row 13",
		},
		{
			fileName:     "no_header_row_account",
			errorMessage: "after scanning 24 rows can't find headers 'ԱմսաթիվԳումարԱրժույթՄուտքԵլք'",
		},
		{
			fileName:     "transaction_without_debit_and_credit_account",
			errorMessage: "failed to parse debit amount from cell 6 of 22 row: invalid money format: ''",
		},
	}

	for _, tt := range tests {
		t.Run(tt.fileName, func(t *testing.T) {
			parser := AcbaRegularAccountExcelFileParser{}
			filePath := filepath.Join("testdata", "acba", tt.fileName+".xls")
			_, err := parser.ParseRawTransactionsFromFile(filePath)
			if err == nil {
				t.Errorf("expected error, got nil")
			} else if !strings.Contains(err.Error(), tt.errorMessage) {
				t.Errorf("expected error containing %q, got %v", tt.errorMessage, err)
			}
		})
	}
}
