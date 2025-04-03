package main

import (
	"strings"
	"testing"
	"time"
)

func TestAmeriaCsvFileParser_ParseRawTransactionsFromFile_InvalidFilePath(t *testing.T) {
	_, _, err := AmeriaCsvFileParser{}.ParseRawTransactionsFromFile(
		"testdata/ameria/not_existing_path.csv",
	)
	if !strings.Contains(err.Error(), "failed to open file") {
		t.Errorf("expected 'failed to open file' error, but got: %v", err)
	}
}

func TestAmeriaCsvFileParser_ParseRawTransactionsFromFile_BOMInHeader(t *testing.T) {
	filePath := "testdata/ameria/with_bom_header.csv"
	sourceType := "AmeriaCsv:AMD"
	transactions, returnedSourceType, err := AmeriaCsvFileParser{}.ParseRawTransactionsFromFile(filePath)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if returnedSourceType != sourceType {
		t.Errorf("expected source type %s, but got %s", sourceType, returnedSourceType)
	}
	expectedTransactions := []Transaction{
		{
			IsExpense:       false,
			Date:            time.Date(2024, time.May, 17, 0, 0, 0, 0, time.UTC),
			Details:         "Ք: Քարտից քարտ փոխանցում\\",
			Amount:          MoneyWith2DecimalPlaces{int: 20000000},
			SourceType:      sourceType,
			Source:          filePath,
			AccountCurrency: "AMD",
			FromAccount:     "1234567890123456",
			ToAccount:       "9999999999999999",
		},
		{
			IsExpense:       true,
			Date:            time.Date(2024, time.May, 20, 0, 0, 0, 0, time.UTC),
			Details:         "Ք: SOME TEXT",
			Amount:          MoneyWith2DecimalPlaces{int: 55000},
			SourceType:      sourceType,
			Source:          filePath,
			AccountCurrency: "AMD",
			FromAccount:     "9999999999999999",
			ToAccount:       "123456",
		},
	}

	for i, transaction := range transactions {
		if transaction != expectedTransactions[i] {
			t.Errorf("expected transaction %+v, but got %+v", expectedTransactions[i], transaction)
		}
	}
}

func TestAmeriaCsvFileParser_ParseRawTransactionsFromFile_InvalidHeader(t *testing.T) {

	// Act
	_, _, err := AmeriaCsvFileParser{}.ParseRawTransactionsFromFile(
		"testdata/ameria/invalid_header.csv",
	)

	// Assert
	if err == nil {
		t.Errorf("expected an error for invalid header, but got none")
	} else if !strings.Contains(err.Error(), "failed to read line 14: EOF") {
		t.Errorf("expected 'unexpected header' error, but got: %v", err)
	}
}
