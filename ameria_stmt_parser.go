package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/tealeg/xlsx"
)

const giveUpFindHeaderInAmeriaExcelStmtAfterRows = 18
const MyAmeriaStmtDateFormat = "02.01.2006"

var (
	// Headers which exists in all files. Doesn't include "Amount" which are different from file to file.
	xlsHeaders = []string{
		"Date", "Account", "Recipient/Sender", "Operation Type", "Purpose",
	}
)

type MyAmeriaStmtTransaction struct {
	Date                 time.Time
	Account              string
	RecipientOrSender    string
	OperationType        string
	Purpose              string
	Currency             string
	CreditOriginCurrency MoneyWith2DecimalPlaces
	CreditAMD            MoneyWith2DecimalPlaces
	DebitOriginCurrency  MoneyWith2DecimalPlaces
	DebitAMD             MoneyWith2DecimalPlaces
}

type MyAmeriaExcelStmtFileParser struct {
}

func (p MyAmeriaExcelStmtFileParser) ParseRawTransactionsFromFile(
	filePath string,
) ([]Transaction, error) {
	f, err := xlsx.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Find first sheet.
	firstSheet := f.Sheets[0]
	log.Println(i18n.T("file parsing first sheet s from n sheets", "file", filePath, "s", firstSheet.Name, "n", len(f.Sheets)))

	// Parse myAmeriaStmtTransactions.
	var myAmeriaStmtTransactions []MyAmeriaStmtTransaction
	var accountNumber = ""
	var accountCurrency = ""
	var isHeaderRowFound bool
	var creditColumnIndex = -1
	var creditAmdColumnIndex = -1
	var debitColumnIndex = -1
	var debitAmdColumnIndex = -1
	for i, row := range firstSheet.Rows {
		cells := row.Cells
		if len(cells) < len(xlsHeaders) {
			return nil, fmt.Errorf(
				"%d row has only %d cells while need to find information for headers %v",
				i, len(cells), xlsHeaders,
			)
		}
		// Find header row.
		if !isHeaderRowFound {
			if i > giveUpFindHeaderInAmeriaExcelStmtAfterRows {
				return nil, fmt.Errorf(
					"after scanning %d rows can't find headers %v",
					i, xlsHeaders,
				)
			}

			// Try to find account number and currency first.
			if len(accountNumber) < 1 {
				if cells[0].String() == "Account No" {
					// Account number contains extra "'" character.
					accountNumber = strings.Trim(cells[2].String(), "'")
				}
			}
			if len(accountCurrency) < 1 {
				// Currency is placed under "Overdraft current limit" and "Overdraft used amount" labels.
				if cells[0].String() == "Overdraft current limit" {
					// Currency cell contains extra spaces.
					accountCurrency = strings.TrimSpace(cells[2].String())
				}
			}

			var isCellMatches = true
			for cellIndex, header := range xlsHeaders {
				if strings.TrimSpace(cells[cellIndex].String()) != header {
					isCellMatches = false
					break
				}
			}
			if isCellMatches {
				isHeaderRowFound = true
				// This row contains also headers for "Credit XXX" and "Debit XXX" columns.
				// Search indexes of these columns.
				for cellIndex, cell := range cells {
					header := cell.String()
					if header == "Credit "+accountCurrency {
						creditColumnIndex = cellIndex
						continue // Skip this row to avoid getting "creditAmdColumnIndex" set if account currency is AMD.
					}
					if header == "Credit AMD" {
						creditAmdColumnIndex = cellIndex
						continue
					}
					if header == "Debit "+accountCurrency {
						debitColumnIndex = cellIndex
						continue // Skip this row to avoid getting "debitAmdColumnIndex" set if account currency is AMD.
					}
					if header == "Debit AMD" {
						debitAmdColumnIndex = cellIndex
						continue
					}
				}
				if creditColumnIndex == -1 && debitColumnIndex == -1 {
					return nil, fmt.Errorf("%d row has no credit or debit columns", i)
				}
			}

			// Skip this row anyway.
			continue
		}

		// Stop if row doesn't have enough cells or first cell is empty.
		if len(cells) < len(xlsHeaders) || cells[0].String() == "" {
			break
		}

		// Parse date and amounts.
		date, err := time.Parse(MyAmeriaStmtDateFormat, cells[0].String())
		if err != nil {
			return nil, fmt.Errorf("failed to parse date from 1st cell of %d row: %w", i, err)
		}
		var creditAmount MoneyWith2DecimalPlaces
		var creditAmdAmount MoneyWith2DecimalPlaces
		var debitAmount MoneyWith2DecimalPlaces
		var debitAmdAmount MoneyWith2DecimalPlaces
		if creditColumnIndex != -1 && cells[creditColumnIndex].String() != "" {
			creditAmount, err = parseAmountWithoutLetters(cells[creditColumnIndex])
			if err != nil {
				return nil, fmt.Errorf("failed to parse credit amount from cell %d of %d row: %w", creditColumnIndex+1, i+1, err)
			}
		}
		if creditAmdColumnIndex != -1 && cells[creditAmdColumnIndex].String() != "" {
			creditAmdAmount, err = parseAmountWithoutLetters(cells[creditAmdColumnIndex])
			if err != nil {
				return nil, fmt.Errorf("failed to parse credit AMD amount from cell %d of %d row: %w", creditAmdColumnIndex+1, i+1, err)
			}
		}
		if debitColumnIndex != -1 && cells[debitColumnIndex].String() != "" {
			debitAmount, err = parseAmountWithoutLetters(cells[debitColumnIndex])
			if err != nil {
				return nil, fmt.Errorf("failed to parse debit amount from cell %d of %d row: %w", debitColumnIndex+1, i+1, err)
			}
		}
		if debitAmdColumnIndex != -1 && cells[debitAmdColumnIndex].String() != "" {
			debitAmdAmount, err = parseAmountWithoutLetters(cells[debitAmdColumnIndex])
			if err != nil {
				return nil, fmt.Errorf("failed to parse debit AMD amount from cell %d of %d row: %w", debitAmdColumnIndex+1, i+1, err)
			}
		}
		// Build MyAmeria Statement transaction.
		myAmeriaStmtTransactions = append(myAmeriaStmtTransactions, MyAmeriaStmtTransaction{
			Date:                 date,
			Account:              cells[1].String(),
			RecipientOrSender:    cells[2].String(),
			OperationType:        cells[3].String(),
			Purpose:              cells[4].String(),
			Currency:             accountCurrency,
			CreditOriginCurrency: creditAmount,
			CreditAMD:            creditAmdAmount,
			DebitOriginCurrency:  debitAmount,
			DebitAMD:             debitAmdAmount,
		})
	}

	// Convert MyAmeria rows to unified transactions and separate expenses from incomes.
	// Keep AMD as "originCurrency" for case when account currency is AMD and values are provided in "Credit AMD" or "Debit AMD" columns.
	transactions := make([]Transaction, 0, len(myAmeriaStmtTransactions))
	for _, t := range myAmeriaStmtTransactions {
		isExpense := false
		amount := t.CreditOriginCurrency
		originCurrencyAmount := t.CreditAMD
		to := accountNumber
		from := t.Account
		// Expenses has value in "Debit XXX" columns, incomes has value in "Credit XXX" columns.
		if t.DebitOriginCurrency.int > 0 || t.DebitAMD.int > 0 {
			isExpense = true
			to = t.Account
			from = accountNumber
			amount = t.DebitOriginCurrency
			originCurrencyAmount = t.DebitAMD
		}
		// Skip transactions without any amount.
		if amount.int == 0 && originCurrencyAmount.int == 0 {
			continue
		}
		transactions = append(transactions, Transaction{
			IsExpense:            isExpense,
			Date:                 t.Date,
			Details:              t.Purpose,
			SourceType:           "MyAmeriaExcelStatement",
			Source:               filePath,
			AccountCurrency:      accountCurrency,
			Amount:               amount,
			OriginCurrency:       t.Currency,
			OriginCurrencyAmount: originCurrencyAmount,
			FromAccount:          from,
			ToAccount:            to,
		})
	}

	return transactions, nil
}

// parseAmountWithoutLetters parses the amount from the cell value containing currency.
func parseAmountWithoutLetters(cell *xlsx.Cell) (MoneyWith2DecimalPlaces, error) {
	var amount MoneyWith2DecimalPlaces
	value := strings.TrimSpace(cell.String())

	// Find the first consecutive [0-9.-] characters
	var numberStr strings.Builder
	for _, char := range value {
		if (char >= '0' && char <= '9') || char == '.' || char == '-' {
			numberStr.WriteRune(char)
		}
	}

	if numberStr.Len() == 0 {
		return amount, fmt.Errorf("invalid money format: '%s'", value)
	}

	// Parse the amount from the extracted number string
	err := amount.ParseString(numberStr.String())
	if err != nil {
		return amount, fmt.Errorf("failed to parse amount: %w", err)
	}
	return amount, nil
}
