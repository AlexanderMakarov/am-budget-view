package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// Expected headers in the exact order matching Transaction struct fields
var expectedHeaders = []string{
	"Date",
	"FromAccount",
	"ToAccount",
	"IsExpense",
	"Amount",
	"Details",
	"AccountCurrency",
	"OriginCurrency",
	"OriginCurrencyAmount",
}

type GenericCsvFileParser struct{}

func (p GenericCsvFileParser) ParseRawTransactionsFromFile(
	filePath string,
) ([]Transaction, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read the file into a byte slice
	fileData, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Try to decode as UTF-8 first
	reader := csv.NewReader(bytes.NewReader(fileData))
	reader.FieldsPerRecord = -1 // Allow variable number of fields per record

	// Read and validate headers
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read headers: %w", err)
	}

	// Clean headers (trim spaces and quotes)
	for i, header := range headers {
		headers[i] = strings.TrimSpace(strings.Trim(header, `"`))
	}
	// Remove BOM if present
	if len(headers) > 0 {
		headers[0] = strings.TrimPrefix(headers[0], "\ufeff")
	}

	// Validate headers
	if len(headers) != len(expectedHeaders) {
		return nil, fmt.Errorf(
			"incorrect number of headers: got %d, want %d",
			len(headers),
			len(expectedHeaders),
		)
	}
	for i, header := range headers {
		if header != expectedHeaders[i] {
			return nil, fmt.Errorf(
				"incorrect header at position %d: got '%s', want '%s'",
				i+1,
				header,
				expectedHeaders[i],
			)
		}
	}

	sourceType := TransactionsSource{
		TypeName: "Generic CSV with transactions",
		FilePath: filePath,
	}

	var transactions []Transaction
	lineNum := 2 // Start from 2 as line 1 is headers

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading line %d: %w", lineNum, err)
		}

		// Validate record length
		if len(record) != len(expectedHeaders) {
			return nil, fmt.Errorf(
				"incorrect number of fields at line %d: got %d, want %d",
				lineNum,
				len(record),
				len(expectedHeaders),
			)
		}

		// Parse each field
		transaction := Transaction{
			Source: &sourceType,
		}

		// Date (time.Time)
		date, err := time.Parse("2006-01-02", strings.TrimSpace(record[0]))
		if err != nil {
			return nil, fmt.Errorf(
				"line %d: invalid Date format '%s', expected YYYY-MM-DD: %w",
				lineNum,
				record[0],
				err,
			)
		}
		transaction.Date = date

		// FromAccount (string)
		transaction.FromAccount = strings.TrimSpace(record[1])

		// ToAccount (string)
		transaction.ToAccount = strings.TrimSpace(record[2])

		// IsExpense (bool)
		isExpense, err := strconv.ParseBool(strings.TrimSpace(record[3]))
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid IsExpense value '%s': %w", lineNum, record[3], err)
		}
		transaction.IsExpense = isExpense
		if sourceType.AccountNumber == "" {
			if isExpense {
				sourceType.AccountNumber = transaction.FromAccount
			} else {
				sourceType.AccountNumber = transaction.ToAccount
			}
		}

		// Amount (MoneyWith2DecimalPlaces)
		var amount MoneyWith2DecimalPlaces
		if err := amount.UnmarshalText([]byte(strings.TrimSpace(record[4]))); err != nil {
			return nil, fmt.Errorf("line %d: invalid Amount value '%s': %w", lineNum, record[4], err)
		}
		transaction.Amount = amount

		// Details (string)
		transaction.Details = strings.TrimSpace(record[5])

		// AccountCurrency (string)
		transaction.AccountCurrency = strings.TrimSpace(record[6])
		if transaction.AccountCurrency == "" {
			return nil, fmt.Errorf("line %d: AccountCurrency cannot be empty", lineNum)
		}
		if sourceType.AccountCurrency == "" {
			sourceType.AccountCurrency = transaction.AccountCurrency
			sourceType.Tag = fmt.Sprintf("GenericCsv:%s", transaction.AccountCurrency)
		}

		// OriginCurrency (string)
		transaction.OriginCurrency = strings.TrimSpace(record[7])

		// OriginCurrencyAmount (MoneyWith2DecimalPlaces)
		if originAmount := strings.TrimSpace(record[8]); originAmount != "" {
			var originCurrencyAmount MoneyWith2DecimalPlaces
			if err := originCurrencyAmount.UnmarshalText([]byte(originAmount)); err != nil {
				return nil, fmt.Errorf(
					"line %d: invalid OriginCurrencyAmount value '%s': %w",
					lineNum,
					record[8],
					err,
				)
			}
			transaction.OriginCurrencyAmount = originCurrencyAmount
		}

		// Validate required fields are not empty
		if transaction.FromAccount == "" {
			return nil, fmt.Errorf("line %d: FromAccount cannot be empty", lineNum)
		}
		if transaction.ToAccount == "" {
			return nil, fmt.Errorf("line %d: ToAccount cannot be empty", lineNum)
		}
		if transaction.Details == "" {
			return nil, fmt.Errorf("line %d: Details cannot be empty", lineNum)
		}
		if transaction.AccountCurrency == "" {
			return nil, fmt.Errorf("line %d: AccountCurrency cannot be empty", lineNum)
		}
		if transaction.Amount.int == 0 {
			return nil, fmt.Errorf("line %d: Amount cannot be zero", lineNum)
		}

		transactions = append(transactions, transaction)
		lineNum++
	}

	if len(transactions) == 0 {
		return nil, fmt.Errorf("no transactions found in the file")
	}

	return transactions, nil
}
