package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/shakinm/xlsReader/xls"
)

const (
	giveUpFindHeaderInAcbaCardExcelStmtAfterRows = 27
	acbaCardStmtDateFormat                       = "02.01.2006"
	acbaCardAccountCellPrefix                    = "Հաշվի համար:  "
	acbaCardCurrencyCellPrefix                   = "Հաշվի արժույթ:  "
	// acbaCardXlsHeaders is the header row for transaction data
	acbaCardXlsHeaders        = "Գործարքի ամսաթիվԳործարքի գումարըԱրժույթՄուտքԵլք"
	acbaCardFinishRowContains = "ՎԱՍՏԱԿԱԾ ԵԿԱՄՈՒՏՆԵՐ ԵՎ ԲՈՆՈՒՍՆԵՐ"
	acbaCardFinishRow         = "Քաղվածքի վերջ"
)

type AcbaCardExcelFileParser struct {
}

func (p AcbaCardExcelFileParser) ParseRawTransactionsFromFile(
	filePath string,
) ([]Transaction, error) {
	f, err := xls.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Find first sheet.
	firstSheet, err := f.GetSheet(0)
	if err != nil {
		return nil, fmt.Errorf("failed to get first sheet: %w", err)
	}
	log.Println(i18n.T("file parsing first sheet s from n sheets", "file", filePath, "s", firstSheet.GetName(), "n", f.GetNumberSheets()))

	// Parse native transactions first.
	var transactions []Transaction
	var accountNumber = ""
	var accountCurrency = ""
	var source TransactionsSource
	var isHeaderRowFound bool
	for i := 0; i <= firstSheet.GetNumberRows(); i++ {
		row, err := firstSheet.GetRow(i)
		if err != nil {
			continue
		}

		// Skip if row is empty.
		if row == nil {
			continue
		}

		// Find transactions header row first.
		if !isHeaderRowFound {
			if i > giveUpFindHeaderInAcbaCardExcelStmtAfterRows {
				return nil, fmt.Errorf(
					"after scanning %d rows can't find headers %v",
					i, acbaCardXlsHeaders,
				)
			}

			// Concatenate all values into one big string.
			var builder strings.Builder
			for j := 0; j <= 40; j++ {
				cell, err := row.GetCol(j)
				if err != nil {
					continue
				}
				cellValue := cell.GetString()
				if cellValue != "" {
					builder.WriteString(strings.TrimSpace(cellValue))
				}
			}
			rowString := builder.String()

			// Try to find account number and currency first.
			if len(accountNumber) < 1 {
				if strings.Contains(rowString, acbaCardAccountCellPrefix) {
					start := strings.Index(rowString, acbaCardAccountCellPrefix)
					accountNumber = rowString[start+len(acbaCardAccountCellPrefix):]
					// Extract just the account number (remove any trailing text)
					if spaceIndex := strings.Index(accountNumber, " "); spaceIndex > 0 {
						accountNumber = accountNumber[:spaceIndex]
					}
				}
			}
			if len(accountCurrency) < 1 {
				if strings.Contains(rowString, acbaCardCurrencyCellPrefix) {
					start := strings.Index(rowString, acbaCardCurrencyCellPrefix)
					accountCurrency = rowString[start+len(acbaCardCurrencyCellPrefix):]
					// Extract just the currency (remove any trailing text)
					if spaceIndex := strings.Index(accountCurrency, " "); spaceIndex > 0 {
						accountCurrency = accountCurrency[:spaceIndex]
					}
				}
			}

			isHeaderRowFound = rowString == acbaCardXlsHeaders
			if isHeaderRowFound {
				if len(accountNumber) < 1 || len(accountCurrency) < 1 {
					return nil, fmt.Errorf("can't find account number and/or currency down to row %d", i+1)
				}
				source = TransactionsSource{
					TypeName:        "Acba Card XLS statement",
					Tag:             "AcbaCardExcel:" + accountCurrency,
					FilePath:        filePath,
					AccountNumber:   accountNumber,
					AccountCurrency: accountCurrency,
				}
			}

			// Skip this row anyway.
			continue
		}

		// Get cells.
		cells := row.GetCols()
		// Skip rows which don't have enough cells.
		if len(cells) < 30 {
			continue
		}
		dateStr := cells[2].GetString()

		// Stop on "finish" rows.
		if dateStr == acbaCardFinishRow || strings.Contains(dateStr, acbaCardFinishRowContains) {
			break
		}

		// Skip rows without date cell.
		if dateStr == "" {
			continue
		}

		// Try to parse date.
		date, err := time.Parse(acbaCardStmtDateFormat, dateStr)
		if err != nil {
			// Skip rows without date.
			continue
		}

		// Try to parse amount from cell 5 (amount cell).
		amount := MoneyWith2DecimalPlaces{}
		amountStr := cells[5].GetString()
		err = amount.ParseAmountWithoutLettersFromString(amountStr)
		if err != nil {
			amount.int = 0
		}
		isExpense := true // We always check "Credit" column so start with expense assumption.

		// Try to parse credit amount from cell 8 (credit amount cell).
		creditAmount := MoneyWith2DecimalPlaces{}
		creditStr := cells[8].GetString()
		if creditStr != "" {
			err = creditAmount.ParseAmountWithoutLettersFromString(creditStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse credit amount from cell %d of %d row: %w", 8, i+1, err)
			}
			amount = creditAmount
			isExpense = false
		}

		// If amount was not found yet then both amount and credit cells are empty. Check cell 10 (debit amount cell).
		debitAmount := amount
		if amount.int == 0 {
			err = debitAmount.ParseAmountWithoutLettersFromString(cells[10].GetString())
			if err != nil {
				return nil, fmt.Errorf("failed to parse amount from all cells of %d row: %w", i+1, err)
			}
			// Skip rows without any amount.
			if debitAmount.int == 0 {
				log.Printf("skipping row %d because amount is 0 here\n", i+1)
				continue
			}
			// Convert negative amount to positive.
			debitAmount.int = -debitAmount.int
			// Set amount and isExpense based on debit amount.
			amount = debitAmount
			isExpense = true
		}

		// Build "details" from "Transaction description" and "Transaction place" values.
		details := cells[23].GetString() + " " + cells[29].GetString()
		// Trim trailing comma with LF and spaces.
		details = strings.Trim(details, ",\n")
		details = strings.TrimSpace(details)

		// Try to parse currency. If not provided then use account currency.
		currency := cells[6].GetString() // Currency is in column 6
		if currency == "" {
			currency = accountCurrency
		}

		// Set "FromAccount" and "ToAccount" based on "IsExpense" value.
		from := accountNumber
		to := ""
		if !isExpense {
			to = accountNumber
			from = ""
		}

		// Set "origin currency" fields.
		originCurrency := ""
		originCurrencyAmount := MoneyWith2DecimalPlaces{int: 0}
		// If transaction currency is different from account currency then set "origin currency" fields.
		if currency != accountCurrency {
			originCurrency = currency
			if isExpense {
				originCurrencyAmount = debitAmount
			} else {
				originCurrencyAmount = creditAmount
			}
		}

		// Build native transaction.
		transactions = append(transactions, Transaction{
			Date:                 date,
			FromAccount:          from,
			ToAccount:            to,
			IsExpense:            isExpense,
			Amount:               amount,
			AccountCurrency:      accountCurrency,
			Details:              details,
			OriginCurrency:       originCurrency,
			OriginCurrencyAmount: originCurrencyAmount,
			Source:               &source,
		})
	}

	if !isHeaderRowFound {
		return nil, fmt.Errorf("after scanning %d rows can't find headers %v", firstSheet.GetNumberRows(), acbaCardXlsHeaders)
	}

	return transactions, nil
}
