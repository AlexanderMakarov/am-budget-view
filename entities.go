package main

import (
	"strconv"
	"strings"
	"time"
)

// MoneyWith2DecimalPlaces is a wrapper to parse money from "1,500.00" or "1,500" to 150000.
type MoneyWith2DecimalPlaces struct {
	int
}

// UnmarshalText removes commas and parses string as float.
func (m *MoneyWith2DecimalPlaces) UnmarshalText(text []byte) error {
	sanitizedText := strings.Replace(string(text), ",", "", -1)
	floatVal, err := strconv.ParseFloat(sanitizedText, 64)
	if err != nil {
		return err
	}
	m.int = int(floatVal * 100)
	return nil
}

// OutputDateFormat format for data in outputs.
const OutputDateFormat = "2006-01-02"

// Transaction is a struct representing a single transaction.
type Transaction struct {
	IsExpense bool
	Date      time.Time
	Details   string
	Amount    MoneyWith2DecimalPlaces
	Source    string
	// Extra fields for Beancount
	Currency string
	// FromAccount is an account which pays the transaction, amount is decreasing here.
	FromAccount string
	// ToAccount is an account which receives the transaction, amount is increasing here.
	ToAccount string
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
