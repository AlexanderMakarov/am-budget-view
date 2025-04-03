package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const AmeriaBusinessDateFormat = "02/01/2006"
const giveUpFindHeaderInAmeriaCsvAfterNoHeaderLines = 20

var (
	csvHeaders = []string{
		"Date",
		"Doc.No.",
		"Type",
		"Account",
		"Details",
		"Debit",
		"Credit",
		"Remitter/Beneficiary",
	}
	csvHeadersWithAmd = []string{
		"Date",
		"Doc.No.",
		"Type",
		"Account",
		"Details",
		"Debit",
		"Credit",
		"Remitter/Beneficiary",
		"Debit(AMD)",
		"Credit(AMD)",
	}
)

type AmeriaBusinessTransaction struct {
	Date                time.Time
	DocNo               string
	TransactionType     string
	Account             string
	Credit              MoneyWith2DecimalPlaces
	CreditAmd           MoneyWith2DecimalPlaces
	Debit               MoneyWith2DecimalPlaces
	DebitAmd            MoneyWith2DecimalPlaces
	RemitterBeneficiary string
	Details             string
}

type AmeriaCsvFileParser struct {
}

func (p AmeriaCsvFileParser) ParseRawTransactionsFromFile(
	filePath string,
) ([]Transaction, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Initialize variables for currency and account number
	var currency, accountNumber string

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
	reader.Comma = '\t'         // Assuming the CSV is tab-delimited
	reader.LazyQuotes = true    // Allow the reader to handle bare quotes
	reader.FieldsPerRecord = -1 // Allow a variable number of fields per record

	// Search for account, currency and header.
	csvHeadersStr := strings.Join(csvHeaders, "")
	csvHeadersWithAmdStr := strings.Join(csvHeadersWithAmd, "")
	var headerFound bool
	var header []string
	for i := 0; i < giveUpFindHeaderInAmeriaCsvAfterNoHeaderLines && !headerFound; i++ {
		record, err := reader.Read()
		if err != nil {
			return nil, "", fmt.Errorf("failed to read line %d: %w", i, err)
		}

		// Check for currency and account number in the initial lines
		if len(currency) < 1 && strings.Contains(record[0], "Currency") {
			currency = record[3]
			continue
		}
		if len(accountNumber) < 1 && strings.Contains(record[0], "Account No.") {
			accountNumber = record[3]
			continue
		}

		// Check if the current row is a header row
		currentRowStr := rowCellsToString(record)
		if currentRowStr == csvHeadersStr || currentRowStr == csvHeadersWithAmdStr {
			header = record
			headerFound = true
		}
	}

	if !headerFound || len(currency) < 1 || len(accountNumber) < 1 {
		return nil, "", fmt.Errorf(
			"header/currency/account not found after scanning %d lines",
			giveUpFindHeaderInAmeriaCsvAfterNoHeaderLines,
		)
	}

	// Validate header. Check if CSV contains extra AMD columns.
	headerStr := rowCellsToString(header)
	withAmd := headerStr == csvHeadersWithAmdStr
	if !withAmd {
		if headerStr != csvHeadersStr {
			return nil, "", fmt.Errorf("unexpected header: %s", headerStr)
		}
	}
	transactionLength := len(header)

	// Parse transactions
	var csvTransactions []AmeriaBusinessTransaction
	for {
		record, err := reader.Read()
		if err == io.EOF {
			return nil, "", fmt.Errorf("wrong file format - EOF reached before empty line and 'Days count': %w", err)
		}
		if err != nil {
			return nil, "", fmt.Errorf("error reading record: %w", err)
		}
		if len(record) < transactionLength {
			break // Not a transaction row, stop processing.
		}

		// Strip quotes from each field
		for i := range record {
			record[i] = strings.Trim(record[i], `"`)
		}

		// Skip transactions with empty "Details" field - these are adjustments for not-AMD accounts.
		// I.e. for not-AMD accounts there are transactions without "Details" but with some AMD amounts and Type=RVL.
		if record[4] == "" {
			continue
		}

		// Parse date
		date, err := time.Parse(AmeriaBusinessDateFormat, record[0])
		if err != nil {
			return nil, "", fmt.Errorf("failed to parse date: %w", err)
		}

		// Parse credit and debit.
		// Ameria converts everything to AMD. And takes fees for transactions.
		// Therefore transactions for not AMD accounts may have Type=MSC rows
		// where Credit=0, Debit=0, but Debit(AMD) is non-zero - fees for transaction.
		// Real transaction with both Debit!=0 and Debit(AMD)!=0 goes below with Type=CEX.
		var credit, debit MoneyWith2DecimalPlaces
		if err := credit.UnmarshalText([]byte(record[6])); err != nil {
			return nil, "", fmt.Errorf("failed to parse credit %v: %w", record, err)
		}
		if err := debit.UnmarshalText([]byte(record[5])); err != nil {
			return nil, "", fmt.Errorf("failed to parse debit from %v: %w", record, err)
		}

		// Skip no-amount rows - these are Type=CEX adjustments for not-AMD accounts.
		if credit.int == 0 && debit.int == 0 {
			continue
		}

		transaction := AmeriaBusinessTransaction{
			Date:                date,
			DocNo:               record[1],
			TransactionType:     record[2],
			Account:             record[3],
			Debit:               debit,
			Credit:              credit,
			RemitterBeneficiary: record[7],
			Details:             record[4],
		}
		// If currency is not AMD then use credit and debit in AMD amounts.
		if currency != "AMD" {
			var creditAmd, debitAmd MoneyWith2DecimalPlaces
			if err := debitAmd.UnmarshalText([]byte(record[8])); err != nil {
				return nil, "", fmt.Errorf("failed to parse debit(AMD) %v: %w", record, err)
			}
			transaction.DebitAmd = debitAmd
			if err := creditAmd.UnmarshalText([]byte(record[9])); err != nil {
				return nil, "", fmt.Errorf("failed to parse credit(AMD) %v: %w", record, err)
			}
			transaction.CreditAmd = creditAmd
		}
		csvTransactions = append(csvTransactions, transaction)
	}

	sourceType := fmt.Sprintf("AmeriaCsv:%s", currency)

	// Convert CSV rows to unified transactions and separate expenses from incomes.
	transactions := make([]Transaction, len(csvTransactions))
	for i, t := range csvTransactions {
		// By-default is expense.
		isExpense := true
		amount := t.Debit
		var from string = accountNumber
		var to string = t.Account
		// If debit is empty then it is income.
		if amount.int == 0 {
			isExpense = false
			amount = t.Credit
			from = t.Account
			to = accountNumber
		}
		// Eventually check that transaction is not empty.
		if amount.int == 0 {
			return nil, "", fmt.Errorf("unexpected transaction values parsed on %d line: %+v", i+1, t)
		}
		transactions[i] = Transaction{
			IsExpense:       isExpense,
			Date:            t.Date,
			Details:         t.Details,
			SourceType:      sourceType,
			Source:          filePath,
			Amount:          amount,
			AccountCurrency: currency,
			FromAccount:     from,
			ToAccount:       to,
		}
		// If there is AMD amount then treat it as an "original currency" amount.
		if t.CreditAmd.int > 0 || t.DebitAmd.int > 0 {
			transactions[i].OriginCurrency = "AMD"
			if isExpense {
				transactions[i].OriginCurrencyAmount = t.DebitAmd
			} else {
				transactions[i].OriginCurrencyAmount = t.CreditAmd
			}
		}
	}

	return transactions, sourceType, nil
}

func rowCellsToString(rowCells []string) string {
	for i, cell := range rowCells {
		rowCells[i] = strings.TrimSpace(strings.Trim(cell, `"`))
	}
	// Strip BOM prefix from the first cell if cell is present.
	if len(rowCells) > 0 {
		rowCells[0] = strings.TrimPrefix(rowCells[0], "\ufeff")
	}
	return strings.Join(rowCells, "")
}
