package main

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tealeg/xlsx"
)

const MyAmeriaHistoryDateFormat = "02/01/2006"
const giveUpFindHeaderInAmeriaExcelAfterEmpty1Cells = 15

var (
	xlsxHeaders = []string{
		"Ամսաթիվ",
		"Փաստ N",
		"ԳՏ",
		"Ելքագրվող հաշիվ",
		"Շահառուի հաշիվ",
		"Վճարող/Շահառու",
		"Մանրամասներ",
		"Կարգավիճակ",
		"Մեկնաբանություն",
		"Գումար",
		"Արժույթ",
	}
)

type MyAmeriaTransaction struct {
	Date               time.Time
	FactN              string
	PO                 string
	OutgoingAccount    string
	BeneficiaryAccount string
	PayerOrBeneficiary string
	Details            string
	Status             string
	Comment            string
	Amount             MoneyWith2DecimalPlaces
	Currency           string
}

type MyAmeriaExcelFileParser struct {
	MyAccounts []string
}

func (p MyAmeriaExcelFileParser) ParseRawTransactionsFromFile(
	filePath string,
) ([]Transaction, error) {
	f, err := xlsx.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Find first sheet.
	firstSheet := f.Sheets[0]
	fmt.Printf("%s: parsing first sheet '%s', total %d sheets.\n",
		filePath, firstSheet.Name, len(f.Sheets))

	// Parse myAmeriaTransactions.
	var myAmeriaTransactions []MyAmeriaTransaction
	var isHeaderRowFound bool
	for i, row := range firstSheet.Rows {
		cells := row.Cells
		if len(cells) < len(xlsxHeaders) {
			return nil, fmt.Errorf(
				"%d row has only %d cells while need to find information for headers %v",
				i, len(cells), xlsxHeaders,
			)
		}
		// Find header row.
		if !isHeaderRowFound {
			if i > giveUpFindHeaderInAmeriaExcelAfterEmpty1Cells {
				return nil, fmt.Errorf(
					"after scanning %d rows can't find headers %v",
					i, xlsxHeaders,
				)
			}
			var isCellMatches = true
			for cellIndex, header := range xlsxHeaders {
				if strings.TrimSpace(cells[cellIndex].String()) != header {
					isCellMatches = false
					break
				}
			}
			if isCellMatches {
				isHeaderRowFound = true
			}

			// Skip this row anyway.
			continue
		}

		// Stop if row doesn't have enough cells or first cell is empty.
		if len(cells) < len(xlsxHeaders) || cells[0].String() == "" {
			break
		}

		// Parse date and amount.
		date, err := time.Parse(MyAmeriaHistoryDateFormat, cells[0].String())
		if err != nil {
			return nil, fmt.Errorf("failed to parse date from 1st cell of %d row: %w", i, err)
		}
		var amount MoneyWith2DecimalPlaces
		if err := amount.UnmarshalText([]byte(cells[9].String())); err != nil {
			return nil, fmt.Errorf("failed to parse amount from 10th cell of %d row: %w", i, err)
		}

		transaction := MyAmeriaTransaction{
			Date:               date,
			FactN:              cells[1].String(),
			PO:                 cells[2].String(),
			OutgoingAccount:    cells[3].String(),
			BeneficiaryAccount: cells[4].String(),
			PayerOrBeneficiary: cells[5].String(),
			Details:            cells[6].String(),
			Status:             cells[7].String(),
			Comment:            cells[8].String(),
			Amount:             amount,
			Currency:           cells[10].String(),
		}
		myAmeriaTransactions = append(myAmeriaTransactions, transaction)
	}

	// Convert MyAmeria rows to unified transactions and separate expenses from incomes.
	transactions := make([]Transaction, len(myAmeriaTransactions))
	for i, t := range myAmeriaTransactions {
		isExpense := true
		if len(p.MyAccounts) > 0 {
			if slices.Contains(p.MyAccounts, t.BeneficiaryAccount) {
				isExpense = false
			}
		}
		transactions[i] = Transaction{
			IsExpense: isExpense,
			Date:      t.Date,
			Details:   t.Details,
			// Currency is different for each transaction.
			SourceType: fmt.Sprintf("MyAmeriaExcel:%s", t.Currency),
			Source:     filePath,
			// Ameria XLS files show only original currency amount.
			OriginCurrency:       t.Currency,
			OriginCurrencyAmount: t.Amount,
			FromAccount:          t.OutgoingAccount,
			ToAccount:            t.BeneficiaryAccount,
		}
	}

	return transactions, nil
}
