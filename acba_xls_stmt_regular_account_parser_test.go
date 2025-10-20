package main

import (
	"path/filepath"
	// "strings"
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

// func TestAcbaRegularAccountXlsxFileParser_ParseRawTransactionsFromFile_Errors(t *testing.T) {

// 	tests := []struct {
// 		fileName     string
// 		errorMessage string
// 	}{
// 		{
// 			fileName:      "no_data",
// 			errorMessage:  "can't find sheet with name 'Account ENG'",
// 		},
// 		{
// 			fileName:     "no_account_number",
// 			errorMessage: "failed to find any data in the file",
// 		},
// 		{
// 			fileName:     "no_account_currency",
// 			errorMessage: "failed to parse account currency under label 'Account currency: ' after transactions header is found in 12 row",
// 		},
// 		{
// 			fileName:     "no_header_row",
// 			errorMessage: "after scanning 29 rows can't find both headers 'TransactionTransaction amount in card currencyExchange rateDrawn-down dateClosing balanceSender/ReceiverTransaction details' and 'DateAmountCurrencyCreditsDebits'",
// 		},
// 		{
// 			fileName:     "transaction_without_debit_and_credit",
// 			errorMessage: "failed to parse amount from cell 4 of 17 row: strconv.ParseFloat: parsing \"\": invalid syntax",
// 		},
// 		{
// 			fileName:     "transaction_without_account_number_in_senderreceiver",
// 			errorMessage: "failed to parse peer's account number from 16 row: *ARCA something",
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.fileName, func(t *testing.T) {
// 			parser := ArdshinXlsxFileParser{}
// 			filePath := filepath.Join("testdata", "ardshin", tt.fileName+".xlsx")
// 			_, err := parser.ParseRawTransactionsFromFile(filePath)
// 			if err == nil {
// 				t.Errorf("expected error, got nil")
// 			} else if !strings.Contains(err.Error(), tt.errorMessage) {
// 				t.Errorf("expected error containing %q, got %v", tt.errorMessage, err)
// 			}
// 		})
// 	}
// }
