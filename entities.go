package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tealeg/xlsx"
)

// MoneyWith2DecimalPlaces is a wrapper to parse money from "1,500.00" or "1,500" to 150000.
type MoneyWith2DecimalPlaces struct {
	int
}

// ParseString removes commas and parses string as float.
func (m *MoneyWith2DecimalPlaces) ParseString(s string) error {
	sanitizedText := strings.Replace(s, ",", "", -1)
	floatVal, err := strconv.ParseFloat(sanitizedText, 64)
	if err != nil {
		return err
	}
	m.int = int(floatVal * 100)
	return nil
}

// UnmarshalText removes commas and parses string as float.
func (m *MoneyWith2DecimalPlaces) UnmarshalText(text []byte) error {
	return m.ParseString(string(text))
}

// UnmarshalFromExcelCell removes commas and parses cell's string value as float.
func (m *MoneyWith2DecimalPlaces) UnmarshalFromExcelCell(cell *xlsx.Cell) error {
	if len(cell.Value) < 1 {
		return nil
	}
	return m.ParseString(cell.Value)
}

// MarshalJSON implements the json.Marshaler interface.
func (m MoneyWith2DecimalPlaces) MarshalJSON() ([]byte, error) {
	intPart := m.int / 100
	fracPart := m.int % 100

	intStr := strconv.Itoa(intPart)
	fracStr := fmt.Sprintf("%02d", fracPart)

	// Add thousands separator
	var parts []string
	for i := len(intStr); i > 0; i -= 3 {
		if i-3 > 0 {
			parts = append([]string{intStr[i-3 : i]}, parts...)
		} else {
			parts = append([]string{intStr[:i]}, parts...)
		}
	}

	formattedValue := strings.Join(parts, " ") + "." + fracStr
	return json.Marshal(formattedValue)
}

// OutputDateFormat format for data in outputs.
const OutputDateFormat = "2006-01-02"

// Transaction is a struct representing a single transaction.
type Transaction struct {
	// IsExpense is true if transaction is an expense, false if it is an income.
	IsExpense bool
	// Date of the transaction.
	Date time.Time
	// Details is a description of the transaction.
	Details string
	// Amount in account currency.
	Amount MoneyWith2DecimalPlaces
	// SourceType is a type of the source of the transaction. No spaces.
	SourceType string
	// Source identifier, usually file path.
	Source string

	// Extra fields for Beancount. May be empty - depends on source.

	// AccountCurrency is a currency of the account.
	AccountCurrency string
	// OriginCurrency is a currency of the transaction before conversion.
	OriginCurrency string
	// OriginCurrencyAmount is an amount in origin currency.
	OriginCurrencyAmount MoneyWith2DecimalPlaces
	// FromAccount is an account which pays the transaction, amount is decreasing here.
	FromAccount string
	// ToAccount is an account which receives the transaction, amount is increasing here.
	ToAccount string
}

type FileParser interface {
	// ParseRawTransactionsFromFile parses raw/unified transactions
	// from the specified by path file.
	// Returns list of parsed transactions and account number on success or error if can't parse.
	ParseRawTransactionsFromFile(filePath string) ([]Transaction, error)
}

// Group is a struct representing a group of transactions.
type Group struct {
	Name         string
	Total        MoneyWith2DecimalPlaces
	Transactions []Transaction
}

// IntervalStatistics is a struct representing a list of transactions for time interval, usually month.
type IntervalStatistic struct {
	Start   time.Time
	End     time.Time
	Income  map[string]*Group
	Expense map[string]*Group
}

type GroupData struct {
	Total Money
	// other fields...
}

type Money struct {
	Amount int64 `json:"amount"`
}
