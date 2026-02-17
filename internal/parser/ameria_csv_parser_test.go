package parser

import (
	"github.com/AlexanderMakarov/am-budget-view/internal/model"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var moneyComparer = cmp.Comparer(func(x, y model.MoneyWith2DecimalPlaces) bool {
	return x.Cents == y.Cents
})

var diffOnlyTransformer = cmpopts.AcyclicTransformer("diffOnly", func(x model.Transaction) map[string]interface{} {
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
	source := &model.TransactionsSource{
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

	expectedTransactions := []model.Transaction{
		{
			IsExpense:       false,
			Date:            time.Date(2024, time.May, 17, 0, 0, 0, 0, time.UTC),
			Details:         "Ք: Քարտից քարտ փոխանցում\\",
			Amount:          model.MoneyWith2DecimalPlaces{Cents: 20000000},
			Source:          source,
			AccountCurrency: "AMD",
			FromAccount:     "1234567890123456",
			ToAccount:       "9999999999999999",
		},
		{
			IsExpense:       true,
			Date:            time.Date(2024, time.May, 20, 0, 0, 0, 0, time.UTC),
			Details:         "Ք: SOME TEXT",
			Amount:          model.MoneyWith2DecimalPlaces{Cents: 55000},
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

func TestAmeriaCsvFileParser_ParseRawTransactionsFromFile_CommaDelimited(t *testing.T) {
	filePath := "testdata/ameria/comma_delimited.csv"
	source := &model.TransactionsSource{
		TypeName:        "AmeriaBank CSV statement",
		Tag:             "AmeriaCsv:AMD",
		FilePath:        filePath,
		AccountNumber:   "9999999999999999",
		AccountCurrency: "AMD",
	}
	transactions, err := AmeriaCsvFileParser{}.ParseRawTransactionsFromFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedTransactions := []model.Transaction{
		{
			IsExpense:       false,
			Date:            time.Date(2026, time.January, 8, 0, 0, 0, 0, time.UTC),
			Details:         "personal funds transfer",
			Amount:          model.MoneyWith2DecimalPlaces{Cents: 10000000},
			Source:          source,
			AccountCurrency: "AMD",
			FromAccount:     "9999999999999999",
			ToAccount:       "9999999999999999",
		},
		{
			IsExpense:       true,
			Date:            time.Date(2026, time.January, 8, 0, 0, 0, 0, time.UTC),
			Details:         "Ք: YEREVAN CITY AVAN YEREVAN AM 19",
			Amount:          model.MoneyWith2DecimalPlaces{Cents: 202500},
			Source:          source,
			AccountCurrency: "AMD",
			FromAccount:     "9999999999999999",
			ToAccount:       "8888888888888888",
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

func TestAmeriaCsvFileParser_ParseRawTransactionsFromFile_UTF16CommaDelimited(t *testing.T) {
	filePath := "testdata/ameria/utf16_comma_delimited.csv"
	source := &model.TransactionsSource{
		TypeName:        "AmeriaBank CSV statement",
		Tag:             "AmeriaCsv:USD",
		FilePath:        filePath,
		AccountNumber:   "7777777777777777",
		AccountCurrency: "USD",
	}
	transactions, err := AmeriaCsvFileParser{}.ParseRawTransactionsFromFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(transactions))
	}

	expected := model.Transaction{
		IsExpense:       false,
		Date:            time.Date(2024, time.May, 6, 0, 0, 0, 0, time.UTC),
		Details:         "/ROC/5172400124JO///URI/SOME COMPANY LLC/PURPOSE/OTHR",
		Amount:          model.MoneyWith2DecimalPlaces{Cents: 279000},
		Source:          source,
		AccountCurrency: "USD",
		FromAccount:     "26867864",
		ToAccount:       "7777777777777777",
	}

	if diff := cmp.Diff(expected, transactions[0], moneyComparer, diffOnlyTransformer); diff != "" {
		t.Errorf("transaction mismatch (-expected +got):\n%s", diff)
	}
}

func TestAmeriaCsvFileParser_ParseRawTransactionsFromFile_CommaDelimitedUSD(t *testing.T) {
	filePath := "testdata/ameria/comma_delimited_usd.csv"
	source := &model.TransactionsSource{
		TypeName:        "AmeriaBank CSV statement",
		Tag:             "AmeriaCsv:USD",
		FilePath:        filePath,
		AccountNumber:   "7777777777777777",
		AccountCurrency: "USD",
	}
	transactions, err := AmeriaCsvFileParser{}.ParseRawTransactionsFromFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(transactions))
	}

	expected := model.Transaction{
		IsExpense:       false,
		Date:            time.Date(2024, time.May, 6, 0, 0, 0, 0, time.UTC),
		Details:         "/ROC/5172400124JO///URI/SOME COMPANY LLC/PURPOSE/OTHR",
		Amount:          model.MoneyWith2DecimalPlaces{Cents: 279000},
		Source:          source,
		AccountCurrency: "USD",
		FromAccount:     "26867864",
		ToAccount:       "7777777777777777",
	}

	if diff := cmp.Diff(expected, transactions[0], moneyComparer, diffOnlyTransformer); diff != "" {
		t.Errorf("transaction mismatch (-expected +got):\n%s", diff)
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
