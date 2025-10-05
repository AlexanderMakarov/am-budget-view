package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestArdshinXlsxFileParser_ParseRawTransactionsFromFile_Valid(t *testing.T) {
	validFilePath := filepath.Join("testdata", "ardshin", "valid.xlsx")

	accountNumber := "1234567890123456"
	accountCurrency := "AMD"
	source := &TransactionsSource{
		TypeName:        "Ardshin XLS statement",
		Tag:             "ArdshinXlsx:" + accountCurrency,
		FilePath:        validFilePath,
		AccountNumber:   accountNumber,
		AccountCurrency: accountCurrency,
	}

	got, err := ArdshinXlsxFileParser{}.ParseRawTransactionsFromFile(validFilePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []Transaction{
		{
			Date:                 time.Date(2025, time.August, 15, 0, 0, 0, 0, time.UTC),
			IsExpense:            false,
			Amount:               MoneyWith2DecimalPlaces{int: 12300},
			FromAccount:          "2470087380460000",
			ToAccount:            accountNumber,
			Source:               source,
			AccountCurrency:      accountCurrency,
			OriginCurrency:       "",
			OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 0},
			Details:              "MCDONALDS AM LLC 4454********1234 Payment order",
		},
		{
			Date:                 time.Date(2025, time.August, 14, 0, 0, 0, 0, time.UTC),
			IsExpense:            true,
			Amount:               MoneyWith2DecimalPlaces{int: 10800000},
			FromAccount:          accountNumber,
			ToAccount:            "2470010211270000",
			Source:               source,
			AccountCurrency:      accountCurrency,
			OriginCurrency:       "",
			OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 0},
			Details:              "*ARCA 4454********1234 Amount transfer from card to card",
		},
		{
			Date:                 time.Date(2025, time.August, 14, 0, 0, 0, 0, time.UTC),
			IsExpense:            true,
			Amount:               MoneyWith2DecimalPlaces{int: 54000},
			FromAccount:          accountNumber,
			ToAccount:            "2470010211270000",
			Source:               source,
			AccountCurrency:      accountCurrency,
			OriginCurrency:       "",
			OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 0},
			Details:              "*ARCA 4454********1234 Amount transfer from card to card",
		},
		{
			Date:                 time.Date(2025, time.August, 15, 0, 0, 0, 0, time.UTC),
			IsExpense:            true,
			Amount:               MoneyWith2DecimalPlaces{int: 500000},
			FromAccount:          accountNumber,
			ToAccount:            "2470023040920000",
			Source:               source,
			AccountCurrency:      accountCurrency,
			OriginCurrency:       "",
			OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 0},
			Details:              "*Yukom  PBE 4454********1234 Utility payment (B-C online)",
		},
		{
			Date:                 time.Date(2025, time.August, 16, 0, 0, 0, 0, time.UTC),
			IsExpense:            true,
			Amount:               MoneyWith2DecimalPlaces{int: 404910},
			FromAccount:          accountNumber,
			ToAccount:            "2470000348080010",
			Source:               source,
			AccountCurrency:      accountCurrency,
			OriginCurrency:       "USD",
			OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 1049},
			Details:              "*Visa Int. partav.  AMN dolarov 4454********1234 \\840\\123-567-8910\\WL *STEAM PUR",
		},
		{
			Date:                 time.Date(2025, time.August, 18, 0, 0, 0, 0, time.UTC),
			IsExpense:            true,
			Amount:               MoneyWith2DecimalPlaces{int: 681000},
			FromAccount:          accountNumber,
			ToAccount:            "2470010211270000",
			Source:               source,
			AccountCurrency:      accountCurrency,
			OriginCurrency:       "",
			OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 0},
			Details:              "*ARCA 4454********1234 21016919\\AM\\YEREVAN\\ZOVQ BAR",
		},
	}

	if diff := cmp.Diff(expected, got, moneyComparer, diffOnlyTransformer); diff != "" {
		t.Fatalf("transactions mismatch (-expected +got):\n%s", diff)
	}
}

func TestArdshinXlsxFileParser_ParseRawTransactionsFromFile_Errors(t *testing.T) {

	tests := []struct {
		fileName     string
		errorMessage string
	}{
		{
			fileName:      "no_data",
			errorMessage:  "can't find sheet with name 'Account ENG'",
		},
		{
			fileName:     "no_account_number",
			errorMessage: "failed to find any data in the file",
		},
		{
			fileName:     "no_account_currency",
			errorMessage: "failed to parse account currency under label 'Account currency: ' after transactions header is found in 12 row",
		},
		{
			fileName:     "no_header_row",
			errorMessage: "after scanning 29 rows can't find both headers 'TransactionTransaction amount in card currencyExchange rateDrawn-down dateClosing balanceSender/ReceiverTransaction details' and 'DateAmountCurrencyCreditsDebits'",
		},
		{
			fileName:     "transaction_without_debit_and_credit",
			errorMessage: "failed to parse amount from cell 4 of 17 row: strconv.ParseFloat: parsing \"\": invalid syntax",
		},
		{
			fileName:     "transaction_without_account_number_in_senderreceiver",
			errorMessage: "failed to parse peer's account number from 16 row: *ARCA something",
		},
	}

	for _, tt := range tests {
		t.Run(tt.fileName, func(t *testing.T) {
			parser := ArdshinXlsxFileParser{}
			filePath := filepath.Join("testdata", "ardshin", tt.fileName+".xlsx")
			_, err := parser.ParseRawTransactionsFromFile(filePath)
			if err == nil {
				t.Errorf("expected error, got nil")
			} else if !strings.Contains(err.Error(), tt.errorMessage) {
				t.Errorf("expected error containing %q, got %v", tt.errorMessage, err)
			}
		})
	}
}
