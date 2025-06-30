package main

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var moneyComparer = cmp.Comparer(func(x, y MoneyWith2DecimalPlaces) bool {
	return x.int == y.int
})

var diffOnlyTransformer = cmpopts.AcyclicTransformer("diffOnly", func(x Transaction) map[string]interface{} {
	// Create a map with only the fields that differ
	result := make(map[string]interface{})
	if x.Source != nil {
		result["Source"] = x.Source
	}
	return result
})

func TestAmeriaCsvFileParser_ParseRawTransactionsFromFile_InvalidFilePath(t *testing.T) {
	_, err := AmeriaCsvFileParser{}.ParseRawTransactionsFromFile(
		"testdata/ameria/not_existing_path.csv",
	)
	if !strings.Contains(err.Error(), "failed to open file") {
		t.Errorf("expected 'failed to open file' error, but got: %v", err)
	}
}

func TestAmeriaCsvFileParser_ParseRawTransactionsFromFile_BOMInHeader(t *testing.T) {
	filePath := "testdata/ameria/with_bom_header.csv"
	source := &TransactionsSource{
		TypeName:        "AmeriaBank CSV statement",
		Tag:             "AmeriaCsv:AMD",
		FilePath:        filePath,
		AccountNumber:   "9999999999999999",
		AccountCurrency: "AMD",
	}
	transactions, err := AmeriaCsvFileParser{}.ParseRawTransactionsFromFile(filePath)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expectedTransactions := []Transaction{
		{
			IsExpense:       false,
			Date:            time.Date(2024, time.May, 17, 0, 0, 0, 0, time.UTC),
			Details:         "Ք: Քարտից քարտ փոխանցում\\",
			Amount:          MoneyWith2DecimalPlaces{int: 20000000},
			Source:          source,
			AccountCurrency: "AMD",
			FromAccount:     "1234567890123456",
			ToAccount:       "9999999999999999",
		},
		{
			IsExpense:       true,
			Date:            time.Date(2024, time.May, 20, 0, 0, 0, 0, time.UTC),
			Details:         "Ք: SOME TEXT",
			Amount:          MoneyWith2DecimalPlaces{int: 55000},
			Source:          source,
			AccountCurrency: "AMD",
			FromAccount:     "9999999999999999",
			ToAccount:       "123456",
		},
	}

	for i, transaction := range transactions {
		if diff := cmp.Diff(expectedTransactions[i], transaction, moneyComparer, diffOnlyTransformer); diff != "" {
			t.Errorf("transaction %d mismatch (-expected +got):\n%s", i, diff)
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
	} else if !strings.Contains(err.Error(), "failed to read line 14: EOF") {
		t.Errorf("expected 'unexpected header' error, but got: %v", err)
	}
}
