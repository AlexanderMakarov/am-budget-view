package main

import (
	"path/filepath"
	// "strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestAcbaCardExcelFileParser_ParseRawTransactionsFromFile_Valid(t *testing.T) {
	validFilePath := filepath.Join("testdata", "acba", "valid_card.xls")

	accountNumber := "1234567890123456"
	accountCurrency := "AMD"
	source := &TransactionsSource{
		TypeName:        "Acba Card XLS statement",
		Tag:             "AcbaCardExcel:" + accountCurrency,
		FilePath:        validFilePath,
		AccountNumber:   accountNumber,
		AccountCurrency: accountCurrency,
	}

	got, err := AcbaCardExcelFileParser{}.ParseRawTransactionsFromFile(validFilePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []Transaction{
		{
			Date:                 time.Date(2025, time.October, 1, 0, 0, 0, 0, time.UTC),
			IsExpense:            true,
			Amount:               MoneyWith2DecimalPlaces{int: 300000}, // 3,000.00
			FromAccount:          accountNumber,
			ToAccount:            "",
			Source:               source,
			AccountCurrency:      accountCurrency,
			OriginCurrency:       "",
			OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 0},
			Details:              "Յուքոմ բջջ . պարտքի վճար /43210123/ (մոբայլ բանկինց 150123456)",
		},
		{
			Date:                 time.Date(2025, time.October, 1, 0, 0, 0, 0, time.UTC),
			IsExpense:            false,
			Amount:               MoneyWith2DecimalPlaces{int: 1230000}, // 12,300.00
			FromAccount:          "",
			ToAccount:            accountNumber,
			Source:               source,
			AccountCurrency:      accountCurrency,
			OriginCurrency:       "",
			OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 0},
			Details:              "Արժ, գնում RUR Name Surname (մոբայլ բանկինց 150123457)",
		},
		{
			Date:                 time.Date(2025, time.October, 4, 0, 0, 0, 0, time.UTC),
			IsExpense:            true,
			Amount:               MoneyWith2DecimalPlaces{int: 420000}, // 4,200.00
			FromAccount:          accountNumber,
			ToAccount:            "",
			Source:               source,
			AccountCurrency:      accountCurrency,
			OriginCurrency:       "",
			OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 0},
			Details:              "Վճարում   ZOOVET CENTER",
		},
		{
			Date:                 time.Date(2025, time.October, 4, 0, 0, 0, 0, time.UTC),
			IsExpense:            true,
			Amount:               MoneyWith2DecimalPlaces{int: 170000}, // 1,700.00
			FromAccount:          accountNumber,
			ToAccount:            "",
			Source:               source,
			AccountCurrency:      accountCurrency,
			OriginCurrency:       "",
			OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 0},
			Details:              "Էլեկտրոնային վճարում   YANDEX.GO",
		},
		{
			Date:                 time.Date(2025, time.October, 4, 0, 0, 0, 0, time.UTC),
			IsExpense:            true,
			Amount:               MoneyWith2DecimalPlaces{int: 1100000}, // 11,000.00
			FromAccount:          accountNumber,
			ToAccount:            "",
			Source:               source,
			AccountCurrency:      accountCurrency,
			OriginCurrency:       "",
			OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 0},
			Details:              "Վճարում   SIGMA 90 LLC",
		},
		{
			Date:                 time.Date(2025, time.October, 4, 0, 0, 0, 0, time.UTC),
			IsExpense:            true,
			Amount:               MoneyWith2DecimalPlaces{int: 45000}, // 450.00
			FromAccount:          accountNumber,
			ToAccount:            "",
			Source:               source,
			AccountCurrency:      accountCurrency,
			OriginCurrency:       "",
			OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 0},
			Details:              "Էլեկտրոնային վճարում   TELCELL 1",
		},
		{
			Date:                 time.Date(2025, time.October, 4, 0, 0, 0, 0, time.UTC),
			IsExpense:            false,
			Amount:               MoneyWith2DecimalPlaces{int: 440000}, // 4,400.00
			FromAccount:          "",
			ToAccount:            accountNumber,
			Source:               source,
			AccountCurrency:      accountCurrency,
			OriginCurrency:       "",
			OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 0},
			Details:              "Քարտից քարտ փոխանցում, INECOBANK P2P   INECOBANK P2P",
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
