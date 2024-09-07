package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

const AmeriaBusinessDateFormat = "02/01/2006"

var (
	csvHeaders = []string{
		"Date",
		"Transaction Type",
		"Doc.No.",
		"Account",
		"Credit",
		"Debit",
		"Remitter/Beneficiary",
		"Details",
	}
)

type AmeriaBusinessTransaction struct {
	Date                time.Time
	TransactionType     string
	DocNo               string
	Account             string
	Credit              MoneyWith2DecimalPlaces
	Debit               MoneyWith2DecimalPlaces
	RemitterBeneficiary string
	Details             string
}

type AmeriaCsvFileParser struct {
	Currency string
}

func NormilazeAccountName(input string) string {
	reg, err := regexp.Compile("[^a-zA-Z0-9-]+")
	if err != nil {
		panic(err)
	}
	processedString := reg.ReplaceAllString(input, "")
	return processedString
}

func (p AmeriaCsvFileParser) ParseRawTransactionsFromFile(
	filePath string,
) ([]Transaction, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Due to file doesn't have Account information use file name.
	accountNamePlaceholder := fmt.Sprintf("AccountFrom%s", NormilazeAccountName(file.Name()))

	// Read the file into a byte slice
	fileData, err := io.ReadAll(file)
	if err != nil {
		panic(err)
	}

	// Convert UTF-16 to UTF-8
	utf8Data, err := decodeUTF16ToUTF8(fileData)
	if err != nil {
		panic(err)
	}

	reader := csv.NewReader(bytes.NewReader(utf8Data))
	reader.Comma = '\t'      // Assuming the CSV is tab-delimited
	reader.LazyQuotes = true // Allow the reader to handle bare quotes

	// Read the header row
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Strip BOM from the first header field if present
	if len(header) > 0 && strings.HasPrefix(header[0], "\ufeff") {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}

	// Validate header
	for i, h := range csvHeaders {
		if strings.TrimSpace(strings.Trim(header[i], `"`)) != h {
			return nil, fmt.Errorf("unexpected header: got %s, want %s", header[i], h)
		}
	}

	// Parse transactions
	var csvTransactions []AmeriaBusinessTransaction
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}

		// Strip quotes from each field
		for i := range record {
			record[i] = strings.Trim(record[i], `"`)
		}

		// Parse date
		date, err := time.Parse(AmeriaBusinessDateFormat, record[0])
		if err != nil {
			return nil, fmt.Errorf("failed to parse date: %w", err)
		}

		// Parse credit and debit
		var credit, debit MoneyWith2DecimalPlaces
		if err := credit.UnmarshalText([]byte(record[4])); err != nil {
			return nil, fmt.Errorf("failed to parse credit %v: %w", record, err)
		}
		if err := debit.UnmarshalText([]byte(record[5])); err != nil {
			return nil, fmt.Errorf("failed to parse debit from %v: %w", record, err)
		}

		transaction := AmeriaBusinessTransaction{
			Date:                date,
			TransactionType:     record[1],
			DocNo:               record[2],
			Account:             record[3],
			Credit:              credit,
			Debit:               debit,
			RemitterBeneficiary: record[6],
			Details:             record[7],
		}
		csvTransactions = append(csvTransactions, transaction)
	}

	// Convert CSV rows to unified transactions and separate expenses from incomes.
	transactions := make([]Transaction, len(csvTransactions))
	for i, t := range csvTransactions {
		// By-default is expense.
		isExpense := true
		amount := t.Debit
		var from string = accountNamePlaceholder
		var to string = t.Account
		// If debit is empty then it is income.
		if amount.int == 0 {
			isExpense = false
			amount = t.Credit
			// "Account" field always contain foreign account.
			from = t.Account
			to = accountNamePlaceholder
		}
		// Eventually check that transaction is not empty.
		if amount.int == 0 {
			return nil, fmt.Errorf("unexpected transaction values parsed on %d line: %+v", i+1, t)
		}
		transactions[i] = Transaction{
			IsExpense:       isExpense,
			Date:            t.Date,
			Details:         t.Details,
			Source:          filePath,
			Amount:          amount,
			AccountCurrency: p.Currency, // The same, from settings.
			FromAccount:     from,
			ToAccount:       to,
		}
	}

	return transactions, nil
}
