package main

import (
	"strings"
	"testing"
	"time"
)

func TestAmeriaCsvFileParser_ParseRawTransactionsFromFile_InvalidFilePath(t *testing.T) {
	_, err := AmeriaCsvFileParser{}.ParseRawTransactionsFromFile(
		"testdata/ameria/not_existing_path.csv",
	)
	if !strings.Contains(err.Error(), "failed to open file") {
		t.Errorf("expected 'failed to open file' error, but got: %v", err)
	}
}

func TestAmeriaCsvFileParser_ParseRawTransactionsFromFile_BOMInHeader(t *testing.T) {
	transactions, err := AmeriaCsvFileParser{}.ParseRawTransactionsFromFile(
		"testdata/ameria/with_bom_header.csv",
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expectedTransactions := []Transaction{
		{
			IsExpense:   true,
			Date:        time.Date(2024, time.May, 20, 0, 0, 0, 0, time.UTC),
			Details:     "Ք: SOME TEXT",
			Amount:      MoneyWith2DecimalPlaces{int: 55000},
			Currency:    "",
			FromAccount: "1234567890123456",
			ToAccount:   "",
		},
		{
			IsExpense:   false,
			Date:        time.Date(2024, time.May, 17, 0, 0, 0, 0, time.UTC),
			Details:     "Ք: Քարտից քարտ փոխանցում\\",
			Amount:      MoneyWith2DecimalPlaces{int: 20000000},
			Currency:    "",
			FromAccount: "9999999999999999",
			ToAccount:   "",
		},
	}

	for i, transaction := range transactions {
		if transaction != expectedTransactions[i] {
			t.Errorf("expected transaction %v, but got %v", expectedTransactions[i], transaction)
		}
	}
}

func TestAmeriaCsvFileParser_ParseRawTransactionsFromFile_InvalidHeader(t *testing.T) {

	// Act
	_, err := AmeriaCsvFileParser{}.ParseRawTransactionsFromFile(
		"testdata/ameria/invalid_header.csv",
	)

	// Assert
	if err == nil {
		t.Errorf("expected an error for invalid header, but got none")
	} else if !strings.Contains(err.Error(), "unexpected header") {
		t.Errorf("expected 'unexpected header' error, but got: %v", err)
	}
}
