package parser

import (
	"encoding/xml"
	"fmt"
	"github.com/AlexanderMakarov/am-budget-view/internal/model"
	"io"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
)

const InecoDateFormat = "02/01/2006"

type XmlDate struct {
	time.Time
}

type InecoTransaction struct {
	NN                   string                        `xml:"n-n"`
	Number               string                        `xml:"Number"`
	Date                 XmlDate                       `xml:"Date"`
	Currency             string                        `xml:"Currency"`
	Income               model.MoneyWith2DecimalPlaces `xml:"Income"`
	Expense              model.MoneyWith2DecimalPlaces `xml:"Expense"`
	ReceiverPayerAccount string                        `xml:"Receiver-PayerAccount"`
	ReceiverPayer        string                        `xml:"Receiver-Payer"`
	Details              string                        `xml:"Details"`
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

func (InecoXmlParser) ParseRawTransactionsFromFile(
	filePath string,
) ([]model.Transaction, error) {

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

	// Create source.
	source := model.TransactionsSource{
		TypeName:        "Inecobank XML statement",
		Tag:             fmt.Sprintf("InecoXml:%s", stmt.Currency),
		FilePath:        filePath,
		AccountNumber:   stmt.AccountNumber,
		AccountCurrency: stmt.Currency,
	}

	// Conver Inecobank rows to unified transactions.
	transactions := make([]model.Transaction, 0, len(stmt.Operations.Transactions))
	for _, t := range stmt.Operations.Transactions {
		isExpense := t.Income.Cents <= 0
		amount := t.Income.Cents
		var from string
		var to string
		if isExpense {
			from = stmt.AccountNumber
			to = t.ReceiverPayerAccount
			amount = t.Expense.Cents
		} else {
			from = t.ReceiverPayerAccount
			to = stmt.AccountNumber
		}
		transactions = append(transactions, model.Transaction{
			IsExpense: isExpense,
			Date:      t.Date.Time,
			Details:   t.Details,
			// Ineco XML shows amounts only in account currency.
			Amount:          model.MoneyWith2DecimalPlaces{Cents: amount},
			Source:          &source,
			AccountCurrency: t.Currency,
			FromAccount:     from,
			ToAccount:       to,
		})
	}
	return transactions, nil
}

var _ model.FileParser = InecoXmlParser{}
