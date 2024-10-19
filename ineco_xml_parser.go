package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

const InecoDateFormat = "02/01/2006"

type XmlDate struct {
	time.Time
}

type InecoTransaction struct {
	NN                   string                  `xml:"n-n"`
	Number               string                  `xml:"Number"`
	Date                 XmlDate                 `xml:"Date"`
	Currency             string                  `xml:"Currency"`
	Income               MoneyWith2DecimalPlaces `xml:"Income"`
	Expense              MoneyWith2DecimalPlaces `xml:"Expense"`
	ReceiverPayerAccount string                  `xml:"Receiver-PayerAccount"`
	ReceiverPayer        string                  `xml:"Receiver-Payer"`
	Details              string                  `xml:"Details"`
}

type Operations struct {
	Transactions []InecoTransaction `xml:"Operation"`
}

type Statement struct {
	Client         string     `xml:"Client" validate:"required"`
	AccountNumber  string     `xml:"AccountNumber" validate:"required"`
	Currency       string     `xml:"Currency" validate:"required"`
	Period         string     `xml:"Period" validate:"required"`
	OpeningBalance string     `xml:"Openingbalance" validate:"required"`
	ClosingBalance string     `xml:"Closingbalance" validate:"required"`
	Operations     Operations `xml:"Operations" validate:"required"`
}

func (m *MoneyWith2DecimalPlaces) UnmarshalFromXml(d *xml.Decoder, start xml.StartElement) error {
	var v string
	d.DecodeElement(&v, &start)
	v = strings.Replace(v, ",", "", -1)
	floatVal, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return err
	}
	m.int = int(floatVal * 100)
	return nil
}

func (xd *XmlDate) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var v string
	d.DecodeElement(&v, &start)

	parse, err := time.Parse(InecoDateFormat, v)
	if err != nil {
		return err
	}

	xd.Time = parse
	return nil
}

type InecoXmlParser struct {
}

// ParseRawTransactionsFromFile implements FileParser.
func (InecoXmlParser) ParseRawTransactionsFromFile(filePath string) ([]Transaction, error) {

	// Open XML file.
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	// Read the file content
	xmlData, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	// Unmarshal XML.
	var stmt Statement
	err = xml.Unmarshal(xmlData, &stmt)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling XML: %w", err)
	}

	// Validate that all fields are set.
	validate := validator.New()
	for i, operation := range stmt.Operations.Transactions {
		err = validate.Struct(operation)
		if err != nil {
			return nil, fmt.Errorf("error in %d transaction: %w", i+1, err)
		}
	}
	sourceType := fmt.Sprintf("InecoXml:%s", stmt.Currency)

	// Conver Inecobank rows to unified transactions.
	transactions := make([]Transaction, 0, len(stmt.Operations.Transactions))
	for _, t := range stmt.Operations.Transactions {
		isExpense := t.Income.int <= 0
		amount := t.Income.int
		var from string
		var to string
		if isExpense {
			from = stmt.AccountNumber
			to = t.ReceiverPayerAccount
			amount = t.Expense.int
		} else {
			from = t.ReceiverPayerAccount
			to = stmt.AccountNumber
		}
		transactions = append(transactions, Transaction{
			IsExpense: isExpense,
			Date:      t.Date.Time,
			Details:   t.Details,
			// Ineco XML shows amounts only in account currency.
			Amount:          MoneyWith2DecimalPlaces{amount},
			SourceType:      sourceType,
			Source:          filePath,
			AccountCurrency: t.Currency,
			FromAccount:     from,
			ToAccount:       to,
		})
	}
	return transactions, nil
}

var _ FileParser = InecoXmlParser{}
