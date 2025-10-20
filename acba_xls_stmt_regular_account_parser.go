package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/extrame/xls"
)

const (
	giveUpFindHeaderInAcbaAccountExcelStmtAfterRows = 23
	acbaAccountStmtDateFormat                       = "2006-01-02T15:04:05Z"
	acbaAccountCellPrefix                           = "Հաշվի համար՝ "
	acbaCurrencyCellPrefix                          = "Հաշվի արժույթ՝ "
	acbaAccountXlsHeaders                           = "ԱմսաթիվԳումարԱրժույթՄուտքԵլք"
	acbaAccountFinishRowContains                    = "... ..."
	acbaAccountFinishRow                            = "Քաղվածքի վերջ"
)

type AcbaRegularAccountExcelFileParser struct {
}

func (p AcbaRegularAccountExcelFileParser) ParseRawTransactionsFromFile(
	filePath string,
) ([]Transaction, error) {
	f, err := xls.Open(filePath, "utf-8")
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Find first sheet.
	firstSheet := f.GetSheet(0)
	if firstSheet == nil {
		return nil, fmt.Errorf("no sheets found in file")
	}
	log.Println(i18n.T("file parsing first sheet s from n sheets", "file", filePath, "s", firstSheet.Name, "n", f.NumSheets()))

	// Parse native transactions first.
	var transactions []Transaction
	var accountNumber = ""
	var accountCurrency = ""
	var source TransactionsSource
	var isHeaderRowFound bool
	for i := 0; i <= int(firstSheet.MaxRow); i++ {
		row := firstSheet.Row(i)
		if row == nil {
			continue
		}

		// Skip if row is empty.
		if row.LastCol() < 0 {
			continue
		}

		// Find transactions header row first.
		if !isHeaderRowFound {
			if i > giveUpFindHeaderInAcbaAccountExcelStmtAfterRows {
				return nil, fmt.Errorf(
					"after scanning %d rows can't find headers %v",
					i, acbaAccountXlsHeaders,
				)
			}

			// Concatenate all values into one big string.
			var builder strings.Builder
			for j := 0; j <= row.LastCol(); j++ {
				cellValue := row.Col(j)
				builder.WriteString(strings.TrimSpace(cellValue))
			}
			rowString := builder.String()

			// Try to find account number and currency first.
			if len(accountNumber) < 1 {
				if strings.Contains(rowString, acbaAccountCellPrefix) {
					start := strings.Index(rowString, acbaAccountCellPrefix)
					accountNumber = rowString[start+len(acbaAccountCellPrefix):]
					// Extract just the account number (remove any trailing text)
					if spaceIndex := strings.Index(accountNumber, " "); spaceIndex > 0 {
						accountNumber = accountNumber[:spaceIndex]
					}
				}
			}
			if len(accountCurrency) < 1 {
				if strings.Contains(rowString, acbaCurrencyCellPrefix) {
					start := strings.Index(rowString, acbaCurrencyCellPrefix)
					accountCurrency = rowString[start+len(acbaCurrencyCellPrefix):]
					// Extract just the currency (remove any trailing text)
					if spaceIndex := strings.Index(accountCurrency, " "); spaceIndex > 0 {
						accountCurrency = accountCurrency[:spaceIndex]
					}
				}
			}

			isHeaderRowFound = rowString == acbaAccountXlsHeaders
			if isHeaderRowFound {
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

		// Skip if row doesn't have enough cells or cell with date is empty.
		if row.LastCol() < 12 || row.Col(1) == "" {
			continue
		}

		// Stop on "finish" rows.
		dateStr := row.Col(1)
		if dateStr == acbaAccountFinishRow || strings.Contains(dateStr, acbaAccountFinishRowContains) {
			break
		}

		// Try to parse date.
		date, err := time.Parse(acbaAccountStmtDateFormat, dateStr)
		if err != nil {
			// Skip rows without dates.
			continue
		}

		// Parse amount. It is formatted as a string so
		// 1.60 turns to   "1900-06-08000000" string and
		// 144.90 turns to "1939-09-02000000" string.
		// Parse it as a date and then convert to amount.
		amount, err := parseAmountFromExcelDateOrString(row.Col(2))
		if err != nil {
			return nil, fmt.Errorf("failed to parse amount from cell %d of %d row: %w", 2, i+1, err)
		}

		// Parse other amounts as usual numbers.
		creditAmount := MoneyWith2DecimalPlaces{}
		err = creditAmount.ParseAmountWithoutLettersFromString(row.Col(4))
		if err != nil {
			return nil, fmt.Errorf("failed to parse credit amount from cell %d of %d row: %w", 4, i+1, err)
		}
		debitAmount := MoneyWith2DecimalPlaces{}
		err = debitAmount.ParseAmountWithoutLettersFromString(row.Col(6))
		if err != nil {
			return nil, fmt.Errorf("failed to parse debit amount from cell %d of %d row: %w", 6, i+1, err)
		}

		// Try to parse receiver/sender account number from details.
		details := row.Col(12)
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
		isExpense := false
		from := accountNumber
		to := receiverSenderAccountNumber
		originCurrencyAmount := creditAmount
		if debitAmount.int > 0 {
			isExpense = true
			from = receiverSenderAccountNumber
			to = accountNumber
			originCurrencyAmount = debitAmount
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
			OriginCurrency:       row.Col(2),
			OriginCurrencyAmount: originCurrencyAmount,
			Source:               &source,
		})
	}

	return transactions, nil
}

// parseAmountFromExcelDateOrString parses an amount that might be stored as a date in Excel
// or as a regular string. Excel sometimes converts numbers to dates.
func parseAmountFromExcelDateOrString(value string) (MoneyWith2DecimalPlaces, error) {
	amount := MoneyWith2DecimalPlaces{}

	if strings.Contains(value, "T") && strings.Contains(value, "Z") {
		// This is a date that represents a numeric value
		// Excel stores numbers as days since 1900-01-01
		date, err := time.Parse("2006-01-02T15:04:05Z", value)
		if err != nil {
			return amount, fmt.Errorf("failed to parse date amount: %w", err)
		}
		// Convert date to Excel serial number (days since 1900-01-01)
		excelEpoch := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
		daysSinceEpoch := date.Sub(excelEpoch).Hours() / 24
		// Convert to amount (multiply by 100 for cents)
		amount.int = int(daysSinceEpoch * 100)
	} else {
		// Regular numeric parsing
		err := amount.ParseAmountWithoutLettersFromString(value)
		if err != nil {
			return amount, fmt.Errorf("failed to parse amount: %w", err)
		}
	}

	return amount, nil
}
