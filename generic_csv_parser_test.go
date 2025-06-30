package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestGenericCsvFileParser_ParseRawTransactionsFromFile_InvalidFilePath(t *testing.T) {
	_, err := GenericCsvFileParser{}.ParseRawTransactionsFromFile(
		"testdata/generic/not_existing_path.csv",
	)
	if !strings.Contains(err.Error(), "failed to open file") {
		t.Errorf("expected 'failed to open file' error, but got: %v", err)
	}
}

func TestGenericCsvFileParser_ParseRawTransactionsFromFile_Success(t *testing.T) {
	// Create a temporary test file
	content := `Date,FromAccount,ToAccount,IsExpense,Amount,Details,AccountCurrency,OriginCurrency,OriginCurrencyAmount
2024-03-20,123456,789012,true,2.50,Coffee purchase,USD,,
2024-03-21,987654,123456,false,1000.00,Salary deposit,EUR,USD,1100.00`

	tmpfile, err := os.CreateTemp("", "test_*.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Test the parser
	source := &TransactionsSource{
		TypeName:        "Generic CSV with transactions",
		Tag:             "GenericCsv:USD",
		FilePath:        tmpfile.Name(),
		AccountNumber:   "123456",
		AccountCurrency: "USD",
	}
	transactions, err := GenericCsvFileParser{}.ParseRawTransactionsFromFile(tmpfile.Name())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expectedTransactions := []Transaction{
		{
			Date:            time.Date(2024, time.March, 20, 0, 0, 0, 0, time.UTC),
			FromAccount:     "123456",
			ToAccount:       "789012",
			IsExpense:       true,
			Amount:          MoneyWith2DecimalPlaces{int: 250},
			Details:         "Coffee purchase",
			AccountCurrency: "USD",
			Source:          source,
		},
		{
			Date:                 time.Date(2024, time.March, 21, 0, 0, 0, 0, time.UTC),
			FromAccount:          "987654",
			ToAccount:            "123456",
			IsExpense:            false,
			Amount:               MoneyWith2DecimalPlaces{int: 100000},
			Details:              "Salary deposit",
			AccountCurrency:      "EUR",
			OriginCurrency:       "USD",
			OriginCurrencyAmount: MoneyWith2DecimalPlaces{int: 110000},
			Source:               source,
		},
	}

	if len(transactions) != len(expectedTransactions) {
		t.Fatalf("expected %d transactions, got %d", len(expectedTransactions), len(transactions))
	}

	for i, transaction := range transactions {
		if diff := cmp.Diff(expectedTransactions[i], transaction, moneyComparer, diffOnlyTransformer); diff != "" {
			t.Errorf("transaction %d mismatch (-expected +got):\n%s", i, diff)
		}
	}
}

func TestGenericCsvFileParser_ParseRawTransactionsFromFile_InvalidHeaders(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "missing header",
			content: `Date,FromAccount,ToAccount,IsExpense,Amount,Details,AccountCurrency,OriginCurrency
2024-03-20,123456,789012,true,2.50,Coffee purchase,USD,`,
			wantErr: "incorrect number of headers: got 8, want 9",
		},
		{
			name: "wrong header name",
			content: `Date,FromAccount,ToAccount,IsExpense,Amount,Detail,AccountCurrency,OriginCurrency,OriginCurrencyAmount
2024-03-20,123456,789012,true,2.50,Coffee purchase,USD,,`,
			wantErr: "incorrect header at position 6: got 'Detail', want 'Details'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "test_*.csv")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.Write([]byte(tt.content)); err != nil {
				t.Fatal(err)
			}
			if err := tmpfile.Close(); err != nil {
				t.Fatal(err)
			}

			_, err = GenericCsvFileParser{}.ParseRawTransactionsFromFile(tmpfile.Name())
			if err == nil {
				t.Error("expected error, got nil")
			} else if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestGenericCsvFileParser_ParseRawTransactionsFromFile_InvalidData(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "invalid date format",
			content: `Date,FromAccount,ToAccount,IsExpense,Amount,Details,AccountCurrency,OriginCurrency,OriginCurrencyAmount
20-03-2024,123456,789012,true,2.50,Coffee purchase,USD,,`,
			wantErr: "invalid Date format",
		},
		{
			name: "invalid boolean",
			content: `Date,FromAccount,ToAccount,IsExpense,Amount,Details,AccountCurrency,OriginCurrency,OriginCurrencyAmount
2024-03-20,123456,789012,yes,2.50,Coffee purchase,USD,,`,
			wantErr: "invalid IsExpense value",
		},
		{
			name: "invalid amount",
			content: `Date,FromAccount,ToAccount,IsExpense,Amount,Details,AccountCurrency,OriginCurrency,OriginCurrencyAmount
2024-03-20,123456,789012,true,2.5.0,Coffee purchase,USD,,`,
			wantErr: "invalid Amount value",
		},
		{
			name: "empty required field",
			content: `Date,FromAccount,ToAccount,IsExpense,Amount,Details,AccountCurrency,OriginCurrency,OriginCurrencyAmount
2024-03-20,123456,789012,true,2.50,,USD,,`,
			wantErr: "Details cannot be empty",
		},
		{
			name: "invalid origin amount",
			content: `Date,FromAccount,ToAccount,IsExpense,Amount,Details,AccountCurrency,OriginCurrency,OriginCurrencyAmount
2024-03-20,123456,789012,true,2.50,Coffee purchase,USD,EUR,1.2.3`,
			wantErr: "invalid OriginCurrencyAmount value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "test_*.csv")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.Write([]byte(tt.content)); err != nil {
				t.Fatal(err)
			}
			if err := tmpfile.Close(); err != nil {
				t.Fatal(err)
			}

			_, err = GenericCsvFileParser{}.ParseRawTransactionsFromFile(tmpfile.Name())
			if err == nil {
				t.Error("expected error, got nil")
			} else if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestGenericCsvFileParser_ParseRawTransactionsFromFile_EmptyFile(t *testing.T) {
	// Only headers, no data
	content := "Date,FromAccount,ToAccount,IsExpense,Amount,Details,AccountCurrency,OriginCurrency,OriginCurrencyAmount"

	tmpfile, err := os.CreateTemp("", "test_*.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = GenericCsvFileParser{}.ParseRawTransactionsFromFile(tmpfile.Name())
	if err == nil {
		t.Error("expected error for empty file, got nil")
	} else if !strings.Contains(err.Error(), "no transactions found") {
		t.Errorf("expected 'no transactions found' error, got: %v", err)
	}
}
