package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/tealeg/xlsx"
)

const giveUpFindHeaderInInecoExcelAfterRows = 30

var (
	inecoXlsxAccountNumberLabel    = "Հաշվի համար՝"
	inecoXlsxAccountCurrencyLabel  = "Հաշվի արժույթ՝"
	inecoXlsxRegularAccountHeaders = "Գործարքներ/այլ գործառնություններ" +
		"Գործարքի գումար հաշվի արժույթով" +
		"Կիրառվող փոխարժեք" +
		"Հաշվի վերջնական մնացորդ" +
		"Գործարքի նկարագրություն"
	inecoXlsxCardAccountHeaders = "Գործարքներ/այլ գործառնություններ" +
		"Գործարքի գումար հաշվի արժույթով" +
		"Կիրառվող փոխարժեք" +
		"Ձևակերպման (հաշվարկի ապահովման)\n ամսաթիվ" +
		"Հաշվի վերջնական մնացորդ" +
		"Գործարքի նկարագրություն"
	inecoXlsxHeadersBeforeTransactions = "ԱմսաթիվԳումարԱրժույթՄուտքԵլք"
)

type InecoXlsxTransaction struct {
	Date               time.Time
	AmountOrigCur      MoneyWith2DecimalPlaces
	Currency           string
	NotNormalizedEntry MoneyWith2DecimalPlaces
	Income             MoneyWith2DecimalPlaces
	Expense            MoneyWith2DecimalPlaces
	ExchangeRate       MoneyWith2DecimalPlaces
	DateWhenApplied    time.Time
	Details            string
}

type InecoExcelFileParser struct {
}

func (p InecoExcelFileParser) ParseRawTransactionsFromFile(
	filePath string,
) ([]Transaction, error) {
	f, err := xlsx.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Find first sheet.
	firstSheet := f.Sheets[0]
	log.Println(i18n.T("file parsing first sheet s from n sheets", "file", filePath, "s", firstSheet.Name, "n", len(f.Sheets)))

	// Parse Ineco XLSX ransactions.
	var inecoXlsxTransactions []InecoXlsxTransaction
	var accountNumber = ""
	var tag = ""
	var accountCurrency = ""
	var isHeaderRowFound bool
	var isRegularAccount bool
	var prevRowString string
	for i, row := range firstSheet.Rows {
		cells := row.Cells
		// Find header row.
		if !isHeaderRowFound {

			// Sheets has first row empty.
			if len(cells) == 0 {
				continue
			}

			// Note that Ineco XLSX is quite complex with a lot of columns.
			// There are 2 types of XLSX I saw - from regular account and from card account.
			// Regular account XLSX has less columns than card - in card XLSX
			// there is a date of account provision.
			// One more issue - row just before transactions is the same in both cases:
			// "Ամսաթիվ	Գումար		Արժույթ		Մուտք	Ելք"
			// i.e. 5 columns only, unique headers are placed in the row below.
			if i > giveUpFindHeaderInInecoExcelAfterRows {
				return nil, fmt.Errorf(
					"after scanning %d rows can't find headers %v",
					i, inecoXlsxHeadersBeforeTransactions,
				)
			}

			// Due to Ineco XLSX has a lot of columns just concatenate all values into one big string.
			rowString := mergeCellsToString(cells)

			// Try to find account number and currency first.
			if len(accountNumber) < 1 {
				var indexOfAccountNumberLabel = strings.Index(rowString, inecoXlsxAccountNumberLabel)
				if indexOfAccountNumberLabel != -1 {
					accountNumber = rowString[indexOfAccountNumberLabel+len(inecoXlsxAccountNumberLabel):]
					// Remove all "-" characters in the account number.
					accountNumber = strings.ReplaceAll(accountNumber, "-", "")
				}
			}
			if len(accountCurrency) < 1 {
				var indexOfAccountCurrencyLabel = strings.Index(rowString, inecoXlsxAccountCurrencyLabel)
				if indexOfAccountCurrencyLabel != -1 {
					accountCurrency = rowString[indexOfAccountCurrencyLabel+len(inecoXlsxAccountCurrencyLabel):]
				}
			}

			// Check if this row is header row.
			isHeaderRowFound = strings.HasPrefix(rowString, inecoXlsxHeadersBeforeTransactions)
			if isHeaderRowFound {

				// Check if account number and currenct are found.
				if len(accountNumber) < 1 {
					return nil, fmt.Errorf(
						"failed to parse account number under label '%s' after transactions header is found in %d row",
						inecoXlsxAccountNumberLabel, i,
					)
				}
				if len(accountCurrency) < 1 {
					return nil, fmt.Errorf(
						"failed to parse account currency under label '%s' after transactions header is found in %d row",
						inecoXlsxAccountCurrencyLabel, i,
					)
				}

				// Check which XLSX type is by previousRow.
				if strings.HasPrefix(prevRowString, inecoXlsxRegularAccountHeaders) {
					isRegularAccount = true
					tag = fmt.Sprintf("InecoExcelRegular:%s", accountCurrency)
				} else if strings.HasPrefix(prevRowString, inecoXlsxCardAccountHeaders) {
					isRegularAccount = false
					tag = fmt.Sprintf("InecoExcelCard:%s", accountCurrency)
				} else {
					return nil, fmt.Errorf(
						"after scanning %d rows and locating '%s' headers"+
							" can't find either '%s' or '%s' headers (got only '%s') to understand which XLSX type it is",
						i, inecoXlsxHeadersBeforeTransactions, inecoXlsxRegularAccountHeaders, inecoXlsxCardAccountHeaders, prevRowString,
					)
				}

			}
			prevRowString = rowString

			// Skip this row anyway.
			continue
		}

		// Stop if row is empty. Check it before 1st cell to don't skip completely empty row.
		if mergeCellsToString(cells) == "" {
			break
		}

		// Skip all rows with empty first cell - they have only "Final account balance".
		firstCell := cells[0].String()
		if firstCell == "" {
			continue
		}

		// Parse date which is always 1st. Note that it has extra quotes.
		date, err := time.Parse(InecoDateFormat, firstCell)
		if err != nil {
			return nil, fmt.Errorf("failed to parse date from 1st cell of %d row: %w", i, err)
		}
		// Parse other fields of transaction depending on type of XLSX.
		var transaction InecoXlsxTransaction
		if isRegularAccount {
			amountOrigCur, err := parseAmount(i, cells, 1, "amount in original currency")
			if err != nil {
				return nil, fmt.Errorf("%w", err)
			}
			amountIn, err := parseAmount(i, cells, 5, "income amount")
			if err != nil {
				return nil, fmt.Errorf("%w", err)
			}
			amountOut, err := parseAmount(i, cells, 6, "expense amount")
			if err != nil {
				return nil, fmt.Errorf("%w", err)
			}
			rate, err := parseAmount(i, cells, 7, "rate")
			if err != nil {
				return nil, fmt.Errorf("%w", err)
			}

			transaction = InecoXlsxTransaction{
				Date:            date,
				AmountOrigCur:   amountOrigCur,
				Currency:        cells[3].String(),
				Income:          amountIn,
				Expense:         amountOut,
				ExchangeRate:    rate,
				DateWhenApplied: date,
				Details:         cells[17].String(),
			}
		} else {
			dateApplied, err := time.Parse(InecoDateFormat, cells[10].String())
			if err != nil {
				return nil, fmt.Errorf("failed to parse 'date when applied' from 6th cell of %d row: %w", i, err)
			}
			amountOrigCur, err := parseAmount(i, cells, 1, "amount in original currency")
			if err != nil {
				return nil, fmt.Errorf("%w", err)
			}
			amountIn, err := parseAmount(i, cells, 5, "income amount")
			if err != nil {
				return nil, fmt.Errorf("%w", err)
			}
			amountOut, err := parseAmount(i, cells, 6, "expense amount")
			if err != nil {
				return nil, fmt.Errorf("%w", err)
			}
			rate, err := parseAmount(i, cells, 7, "rate")
			if err != nil {
				return nil, fmt.Errorf("%w", err)
			}

			transaction = InecoXlsxTransaction{
				Date:            date,
				AmountOrigCur:   amountOrigCur,
				Currency:        cells[3].String(),
				Income:          amountIn,
				Expense:         amountOut,
				ExchangeRate:    rate,
				DateWhenApplied: dateApplied,
				Details:         cells[19].String(),
			}
		}
		inecoXlsxTransactions = append(inecoXlsxTransactions, transaction)
	}

	source := TransactionsSource{
		TypeName:        "Inecobank XLSX statement",
		Tag:             tag,
		FilePath:        filePath,
		AccountNumber:   accountNumber,
		AccountCurrency: accountCurrency,
	}

	// Conver Inecobank rows to unified transactions.
	transactions := make([]Transaction, 0, len(inecoXlsxTransactions))
	for _, t := range inecoXlsxTransactions {
		isExpense := t.Income.int <= 0
		// Assume is expense.
		from := accountNumber
		to := "UnknownAccount" // Ineco XLSX doesn't have receiver information.
		accountAmount := -t.Expense.int
		// If is income then change values.
		if !isExpense {
			accountAmount = t.Income.int
			from = "UnknownAccount" // Ineco XLSX doesn't have payer information.
			to = accountNumber
		}
		transaction := Transaction{
			IsExpense:            isExpense,
			Date:                 t.Date,
			Details:              t.Details,
			Amount:               MoneyWith2DecimalPlaces{accountAmount},
			OriginCurrencyAmount: MoneyWith2DecimalPlaces{t.AmountOrigCur.int},
			Source:               &source,
			AccountCurrency:      accountCurrency,
			FromAccount:          from,
			ToAccount:            to,
		}
		// Add "origin" currency only if it is different from account currency.
		if t.Currency != "" && t.Currency != accountCurrency {
			transaction.OriginCurrency = t.Currency
		}
		transactions = append(transactions, transaction)
	}
	return transactions, nil
}

func mergeCellsToString(cells []*xlsx.Cell) string {
	var builder strings.Builder
	for _, cell := range cells {
		builder.WriteString(strings.TrimSpace(cell.Value))
	}
	return builder.String()
}

func parseAmount(rowIndex int, cells []*xlsx.Cell, cellIndex int, name string) (MoneyWith2DecimalPlaces, error) {
	var result MoneyWith2DecimalPlaces
	if err := result.UnmarshalFromExcelCell(cells[cellIndex]); err != nil {
		return result, fmt.Errorf(
			"failed to parse amount as '%s' from %d cell of %d row: %w",
			name, rowIndex+1, cellIndex+1, err,
		)
	}
	return result, nil
}

var _ FileParser = InecoXmlParser{}
