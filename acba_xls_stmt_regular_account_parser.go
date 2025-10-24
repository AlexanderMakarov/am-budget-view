package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/shakinm/xlsReader/xls"
)

const (
	giveUpFindHeaderInAcbaAccountExcelStmtAfterRows = 23
	acbaAccountStmtDateFormat                       = "2006-01-02T15:04:05Z"
	acbaAccountAccountCellPrefix                    = "Հաշվի համար՝ "
	acbaAccountCurrencyCellPrefix                   = "Հաշվի արժույթ՝ "
	acbaAccountXlsHeaders                           = "ԱմսաթիվԳումարԱրժույթՄուտքԵլք"
	acbaAccountFinishRowContains                    = "... ..."
	acbaAccountFinishRow                            = "Քաղվածքի վերջ"
)

type AcbaRegularAccountExcelFileParser struct {
}

func (p AcbaRegularAccountExcelFileParser) ParseRawTransactionsFromFile(
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
			if i > giveUpFindHeaderInAcbaAccountExcelStmtAfterRows {
				return nil, fmt.Errorf(
					"after scanning %d rows can't find headers '%s'",
					i, acbaAccountXlsHeaders,
				)
			}

			// Concatenate all values into one big string.
			var builder strings.Builder
			for j := 0; j <= 20; j++ {
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
				if strings.Contains(rowString, acbaAccountAccountCellPrefix) {
					start := strings.Index(rowString, acbaAccountAccountCellPrefix)
					accountNumber = rowString[start+len(acbaAccountAccountCellPrefix):]
					// Extract just the account number (remove any trailing text)
					if spaceIndex := strings.Index(accountNumber, " "); spaceIndex > 0 {
						accountNumber = accountNumber[:spaceIndex]
					}
				}
			}
			if len(accountCurrency) < 1 {
				if strings.Contains(rowString, acbaAccountCurrencyCellPrefix) {
					start := strings.Index(rowString, acbaAccountCurrencyCellPrefix)
					accountCurrency = rowString[start+len(acbaAccountCurrencyCellPrefix):]
					// Extract just the currency (remove any trailing text)
					if spaceIndex := strings.Index(accountCurrency, " "); spaceIndex > 0 {
						accountCurrency = accountCurrency[:spaceIndex]
					}
				}
			}

			isHeaderRowFound = rowString == acbaAccountXlsHeaders
			if isHeaderRowFound {
				if len(accountNumber) < 1 || len(accountCurrency) < 1 {
					return nil, fmt.Errorf("can't find account number and/or currency down to row %d", i+1)
				}
				source = TransactionsSource{
					TypeName:        "Acba Regular Account XLS statement",
					Tag:             "AcbaAccountExcel:" + accountCurrency,
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
		if len(cells) < 13 {
			continue
		}
		dateStr := cells[1].GetString()

		// Stop on "finish" rows.
		if dateStr == acbaAccountFinishRow || strings.Contains(dateStr, acbaAccountFinishRowContains) {
			break
		}

		// Try to parse date - it might be an Excel serial number
		var date time.Time
		if dateFloat := cells[1].GetFloat64(); dateFloat > 0 {
			// Excel serial number - convert to date
			excelEpoch := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
			date = excelEpoch.AddDate(0, 0, int(dateFloat)-2) // Excel has a leap year bug, so subtract 2
		} else {
			// Try to parse as string date
			date, err = time.Parse(acbaAccountStmtDateFormat, dateStr)
			if err != nil {
				// Skip rows without dates.
				continue
			}
		}

		// Parse amount - now we can get proper numeric values
		amount := MoneyWith2DecimalPlaces{}
		if amountInt := cells[2].GetInt64(); amountInt != 0 {
			amount.int = int(amountInt * 100)
		} else {
			// Fallback to string parsing
			err = amount.ParseAmountWithoutLettersFromString(cells[2].GetString())
			if err != nil {
				return nil, fmt.Errorf("failed to parse amount from cell %d of %d row: %w", 2, i+1, err)
			}
		}

		// Parse other amounts as proper numeric values
		creditAmount := MoneyWith2DecimalPlaces{}
		// Parse credit amount from string (it's formatted like "0.00" or "+ 1,449.00")
		err = creditAmount.ParseAmountWithoutLettersFromString(cells[4].GetString())
		if err != nil {
			return nil, fmt.Errorf("failed to parse credit amount from cell %d of %d row: %w", 4, i+1, err)
		}
		debitAmount := MoneyWith2DecimalPlaces{}
		// Parse debit amount from string (it's formatted like "- 1.60" or "0.00")
		err = debitAmount.ParseAmountWithoutLettersFromString(cells[6].GetString())
		if err != nil {
			return nil, fmt.Errorf("failed to parse debit amount from cell %d of %d row: %w", 6, i+1, err)
		}

		// Try to parse receiver/sender account number from details.
		details := cells[12].GetString()
		receiverSenderAccountNumber := ""
		words := strings.Split(details, " ")
		for _, word := range words {
			// Check word consists only of digits.
			isAllDigits := true
			for _, ch := range word {
				if ch < '0' || ch > '9' {
					isAllDigits = false
					break
				}
			}
			if isAllDigits {
				receiverSenderAccountNumber = word
				break
			}
		}

		// Determine if transaction is expense or income.
		currency := cells[3].GetString()
		isExpense := false
		from := accountNumber
		to := receiverSenderAccountNumber
		originCurrencyAmount := creditAmount
		if debitAmount.int != 0 {
			isExpense = true
			from = receiverSenderAccountNumber
			to = accountNumber
			originCurrencyAmount = MoneyWith2DecimalPlaces{int: -debitAmount.int}
		}

		// Clear "origin currency" fields if account currency is used.
		originCurrency := currency
		if currency == accountCurrency {
			originCurrency = ""
			originCurrencyAmount = MoneyWith2DecimalPlaces{int: 0}
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
		return nil, fmt.Errorf("after scanning %d rows can't find headers '%s'", firstSheet.GetNumberRows(), acbaAccountXlsHeaders)
	}

	return transactions, nil
}
