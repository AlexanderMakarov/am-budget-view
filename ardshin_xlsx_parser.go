package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/tealeg/xlsx"
)

const (
	giveUpFindHeaderInArdshinXlsxAfterRows = 28
	ardshinXlsxDateFormat                  = "02.01.2006"
	ardshinEnglishSheetName                = "Account ENG"
	ardshinXlsxAccountNumberLabel          = "Account number:"
	ardshinXlsxAccountCurrencyPrefix       = "Account currency: "
	ardshinXlsxHeaders1String              = "TransactionTransaction amount in card currencyExchange rateDrawn-down dateClosing balanceSender/ReceiverTransaction details"
	ardshinXlsxHeaders2String              = "DateAmountCurrencyCreditsDebits"
)

type ArdshinXlsxFileParser struct {
}

func (p ArdshinXlsxFileParser) ParseRawTransactionsFromFile(
	filePath string,
) ([]Transaction, error) {
	f, err := xlsx.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Find sheet with English.
	var engSheet *xlsx.Sheet
	for _, sheet := range f.Sheets {
		if sheet.Name == ardshinEnglishSheetName {
			engSheet = sheet
			break
		}
	}
	if engSheet == nil {
		return nil, fmt.Errorf("can't find sheet with name '%s'", ardshinEnglishSheetName)
	}
	log.Println(i18n.T("f parsing sheet s from n sheets", "f", filePath, "s", engSheet.Name, "n", len(f.Sheets)))

	var transactions []Transaction
	var accountNumber = ""
	var accountCurrency = ""
	var isHeader1RowFound bool
	var isHeader2RowFound bool
	var source TransactionsSource
	for i, row := range engSheet.Rows {
		cells := row.Cells

		// Skip if row is empty.
		if len(cells) < 1 {
			continue
		}

		// Find transactions header row first.
		if !isHeader2RowFound {

			// Stop if after scanning too many rows transactions header row is not found.
			if i > giveUpFindHeaderInArdshinXlsxAfterRows {
				return nil, fmt.Errorf(
					"after scanning %d rows can't find both headers '%s' and '%s'",
					i, ardshinXlsxHeaders1String, ardshinXlsxHeaders2String,
				)
			}

			// Concatenate all values into one big string.
			var builder strings.Builder
			for _, cell := range cells {
				builder.WriteString(strings.TrimSpace(cell.Value))
			}
			rowString := builder.String()

			// Try to find account number and currency first.
			if len(accountNumber) < 1 {
				var indexOfAccountNumberLabel = strings.Index(rowString, ardshinXlsxAccountNumberLabel)
				if indexOfAccountNumberLabel != -1 {
					accountNumber = rowString[indexOfAccountNumberLabel+len(ardshinXlsxAccountNumberLabel):]
					// Remove all "'" characters in the account number.
					accountNumber = strings.ReplaceAll(accountNumber, "'", "")
				}
			}
			if len(accountCurrency) < 1 {
				var indexOfAccountCurrencyLabel = strings.Index(rowString, ardshinXlsxAccountCurrencyPrefix)
				if indexOfAccountCurrencyLabel != -1 {
					accountCurrency = rowString[indexOfAccountCurrencyLabel+len(ardshinXlsxAccountCurrencyPrefix):]
				}
			}

			// Check if this row is header row.
			if !isHeader1RowFound {
				isHeader1RowFound = rowString == ardshinXlsxHeaders1String
			}
			if isHeader1RowFound {

				// Check if account number and currency are found.
				if len(accountNumber) < 1 {
					return nil, fmt.Errorf(
						"failed to parse account number under label '%s' after transactions header is found in %d row",
						ardshinXlsxAccountNumberLabel, i,
					)
				}
				if len(accountCurrency) < 1 {
					return nil, fmt.Errorf(
						"failed to parse account currency under label '%s' after transactions header is found in %d row",
						ardshinXlsxAccountCurrencyPrefix, i,
					)
				}

				isHeader2RowFound = rowString == ardshinXlsxHeaders2String
				if isHeader2RowFound {
					// Build source.
					source = TransactionsSource{
						TypeName:        "Ardshin XLS statement",
						Tag:             "ArdshinXlsx:" + accountCurrency,
						FilePath:        filePath,
						AccountNumber:   accountNumber,
						AccountCurrency: accountCurrency,
					}
				}
			}

			// Skip this row anyway.
			continue
		}

		// Stop if first cell contains "Total" string.
		firstCellValue := cells[0].String()
		if firstCellValue == "Total" {
			break
		} else if firstCellValue == "" {
			// Skip if first cell is empty - statement has separator rows.
			continue
		}

		// Here should be valid transaction row.
		// Build transaction piece by piece.
		transaction := Transaction{
			Source:          &source,
			AccountCurrency: accountCurrency,
			Details:         cells[14].String(),
		}
		// Parse transaction date.
		date, err := time.Parse(ardshinXlsxDateFormat, cells[0].String())
		if err != nil {
			return nil, fmt.Errorf("failed to parse date from 1st cell of %d row: %w", i, err)
		}
		transaction.Date = date
		// Determine is it expense or income.
		creditCellValue := cells[3].String()
		debitCellValue := cells[5].String()
		transaction.IsExpense = creditCellValue == "" && debitCellValue != ""
		// Set "FromAccount"/"ToAccount" and parse amount in account currency
		// depending on is it expense or income.
		if transaction.IsExpense {
			transaction.FromAccount = accountNumber
			err = transaction.Amount.ParseString(debitCellValue)
			if err != nil {
				return nil, fmt.Errorf("failed to parse amount from cell 6 of %d row: %w", i+1, err)
			}
			// Convert negative amount to positive.
			transaction.Amount.int = -transaction.Amount.int
		} else {
			transaction.ToAccount = accountNumber
			err = transaction.Amount.ParseString(creditCellValue)
			if err != nil {
				return nil, fmt.Errorf("failed to parse amount from cell 4 of %d row: %w", i+1, err)
			}
		}
		// Check we have amount in original currency.
		currencyCellValue := cells[2].String()
		if currencyCellValue != accountCurrency {
			originCurAmount := MoneyWith2DecimalPlaces{}
			err = originCurAmount.ParseString(cells[1].String())
			if err != nil {
				return nil, fmt.Errorf("failed to parse amount from cell 2 of %d row: %w", i+1, err)
			}
			// "Amount" always contains positive amount.
			transaction.OriginCurrencyAmount = originCurAmount
			transaction.OriginCurrency = currencyCellValue
			// FYI: Ardshinbank is the only bank from supported ones that specifies date
			// when exchange was executed ("Drawn-down date" column value),
			// so rate could be different on transaction date.
			// But difference is a few dates and let's sacrifice accuracy for simplicity.
		}
		// "Sender/Receiver" contains not only account number but also some
		// other information which other banks usually put into "Details" field.
		// Therefore we are cutting out account number from "Sender/Receiver" field
		// and putting remainings into "Details" field.
		// Also it looks like Ardshinbank shows only internal account numbers (starts with "2470"),
		// so tracking "transfer my own funds" from other banks looks doomed.
		senderReceiverValue := cells[13].String()
		// Peer's account number is the last sequence of digits in "Sender/Receiver" field.
		// Trim by spaces and the last would be peer's account number.
		words := strings.Split(senderReceiverValue, " ")
		peerAccountNumber := ""
		if len(words) > 0 {
			peerAccountNumber = words[len(words)-1]
			if len(peerAccountNumber) > 0 {
				for _, char := range peerAccountNumber {
					if char < '0' || char > '9' {
						peerAccountNumber = ""
						break
					}
				}
			}
		}
		if len(peerAccountNumber) < 1 {
			return nil, fmt.Errorf("failed to parse peer's account number from %d row: %s", i+1, senderReceiverValue)
		}
		if transaction.IsExpense {
			transaction.ToAccount = peerAccountNumber
		} else {
			transaction.FromAccount = peerAccountNumber
		}
		// Build "Details" field.
		// Replace last word with "Transaction details" string and concatenate with spaces.
		words[len(words)-1] = cells[14].String()
		transaction.Details = strings.Join(words, " ")

		// Append transaction to the list.
		transactions = append(transactions, transaction)
	}

	if !isHeader2RowFound {
		return nil, fmt.Errorf("failed to find any data in the file")
	}
	return transactions, nil
}
