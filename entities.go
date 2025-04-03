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

func (m MoneyWith2DecimalPlaces) StringNoIndent() string {
	dollars := m.int / 100
	cents := m.int % 100
	dollarString := strconv.Itoa(dollars)
	for i := len(dollarString) - 3; i > 0; i -= 3 {
		dollarString = dollarString[:i] + "," + dollarString[i:]
	}
	return fmt.Sprintf("%s.%02d", dollarString, cents)
}

// OutputDateFormat format for data in outputs.
const OutputDateFormat = "2006-01-02"

// Transaction represents a single transaction with data available in the source file.
type Transaction struct {
	// Date of the transaction.
	Date time.Time
	// FromAccount is an account which pays the transaction, amount is decreasing here.
	FromAccount string
	// ToAccount is an account which receives the transaction, amount is increasing here.
	ToAccount string
	// IsExpense is true if transaction is an expense, false if it is an income.
	IsExpense bool
	// Amount in account currency.
	Amount MoneyWith2DecimalPlaces
	// Details is a description of the transaction.
	Details string
	// SourceType is a type of the source of the transaction. No spaces.
	SourceType string
	// Source identifier, usually file path.
	Source string
	// AccountCurrency is a currency of the account. ISO 3-character code.
	AccountCurrency string
	// OriginCurrency is a currency of the transaction before conversion. ISO 3-character code.
	// Can be empty if transaction is in account currency.
	OriginCurrency string
	// OriginCurrencyAmount is an amount in origin currency.
	// Can be empty if transaction is in account currency.
	OriginCurrencyAmount MoneyWith2DecimalPlaces
}

// AccountStatistics is a struct representing data about an account found in transactions.
type AccountStatistics struct {
	// Number is an account number/name.
	Number string
	// IsTransactionAccount flag that account is "from" in expense or "to" in income.
	// I.e. flag that this is an account of the source (file).
	IsTransactionAccount bool
	// SourceType is copied from Transaction.SourceType.
	SourceType string
	// Source is copied from Transaction.Source.
	Source string
	// From is a first transaction date with this account.
	From time.Time
	// To is a last transaction date with this account.
	To time.Time
	// OccurencesInTransactions is a number of transactions with this account.
	OccurencesInTransactions int
}

// AmountInCurrency is an amount in a specific currency with marks of origin and account currencies.
type AmountInCurrency struct {
	Amount MoneyWith2DecimalPlaces
	// Currency name (as in source file but verified by Beancount rules).
	Currency string
	// ConversionPrecision is a number representing how precise conversion was.
	// 0 - no conversion (transaction in this currency),
	// 1 - with direct exchange rate to this currency at the same date,
	// >1 - number of days between transaction date to used exchange rate date, plus the number of days to the next exchange rate if first one was not direct.
	ConversionPrecision int
}

// JournalEntry represents a single transaction with normalized data.
// Normalization means that it has common:
// - category assigned,
// - amount converted into all supported currencies and with marks of origin and account currencies.
type JournalEntry struct {
	// Date of the transaction.
	Date time.Time
	// IsExpense is true if transaction is an expense, false if it is an income.
	IsExpense bool
	// SourceType is a type of the source of the transaction. No spaces.
	SourceType string
	// Source identifier, usually file path.
	Source string
	// Details is a description of the transaction.
	Details string
	// Category is a user-defined and evaluated category of the transaction.
	Category string
	// FromAccount is an account which pays the transaction, amount is decreasing here.
	FromAccount string
	// ToAccount is an account which receives the transaction, amount is increasing here.
	ToAccount string
	// AccountCurrency is a currency of the account.
	AccountCurrency string
	// AccountCurrencyAmount is an amount in account currency.
	AccountCurrencyAmount MoneyWith2DecimalPlaces
	// OriginCurrency is a currency of the transaction before conversion.
	OriginCurrency string
	// OriginCurrencyAmount is an amount in origin currency.
	OriginCurrencyAmount MoneyWith2DecimalPlaces
	// Amounts contains "converted" amounts in given currencies.
	Amounts map[string]AmountInCurrency
}

type FileParser interface {
	// ParseRawTransactionsFromFile parses raw/unified transactions
	// from the specified by path file.
	// Returns:
	// - list of parsed transactions,
	// - file type (source type, no spaces),
	// - error if can't parse.
	ParseRawTransactionsFromFile(filePath string) ([]Transaction, string, error)
}

// Group is a struct representing a group of journal entries.
type Group struct {
	// Name is a name of the group.
	Name string
	// Total is a total amount of the group.
	// May be lower than sum of amounts in journal entries if some entries are not included.
	Total MoneyWith2DecimalPlaces
	// JournalEntries is a list of all journal entries in the group.
	JournalEntries []JournalEntry
}

// IntervalStatistics is a struct representing a list of journal entries for time interval, usually month.
// Contains "income" and "expense" groups of journal entries for one currency.
type IntervalStatistic struct {
	// Currency is a currency of the interval.
	Currency string
	// Start is a start date of the interval.
	Start time.Time
	// End is a end date of the interval.
	End time.Time
	// Income is a map of "income" type `Group`-s.
	Income map[string]*Group
	// Expense is a map of "expense" type `Group`-s.
	Expense map[string]*Group
}
