package main

import (
	"fmt"
	"regexp"
	"time"
)

// https://beancount.github.io/docs/beancount_cheat_sheet.html#beancount-syntax-cheat-sheet
// Need to declare (https://beancount.github.io/docs/beancount_language_syntax.html):
// - Accounts: [Assets Liabilities Equity Income Expenses] - need to open!
// - Commodities / Currencies: uppercased up to 24 characters
// - Transactions: sum always should be zero
// complete:
// 2014-05-05 * "Cafe Mogador" "Lamb tagine with wine"
//   Liabilities:CreditCard:CapitalOne         -37.45 USD
//   Expenses:Restaurant
// incomplte or not sure that correct:
// 2014-05-05 ! "Cafe Mogador" "Lamb tagine with wine"
//   Liabilities:CreditCard:CapitalOne         -37.45 USD
//   Expenses:Restaurant
// convert currency between accounts:
// 2012-11-03 * "Transfer to account in Canada"
//   Assets:MyBank:Checking            -400.00 USD @ 1.09 CAD
//   Assets:FR:SocGen:Checking          436.01 CAD
// transfer with fees:
// 2012-11-03 * "Transfer to account in Canada"
//   Assets:OANDA:GBPounds             -23391.81 GBP
//   Expenses:Fees:WireTransfers           15.00 GBP
//   Assets:Brittania:PrivateBanking    23376.81 GBP

func buildBeanconFile(transactions []Transaction, outputFileName string) error {

	// First iterate all transactions to find all accounts and currencies.
	accounts := make(map[string]time.Time)
	currencies := make(map[string]struct{})
	for _, t := range transactions {
		if validCurrencyRegex.MatchString(t.Currency) {
			currencies[t.Currency] = struct{}{}
		} else {
			return fmt.Errorf("Invalid currency '%s' in transaction: %v", t.Currency, t)
		}
		if len(t.ToAccount) > 0 {
			accounts[t.ToAccount] = t.Date
		}
		if len(t.FromAccount) > 0 {
			accounts[t.FromAccount] = t.Date
		}
	}

	// Next
}

var validCurrencyRegex = regexp.MustCompile(`^[A-Z][A-Z0-9'._-]{0,22}[A-Z0-9]$`)

func (t Transaction) validCurrency() bool {
	// Currency should be uppercased up to 24 characters,
	// and it must start with a capital letter,
	// must end with with a capital letter or number,
	// its other characters must only be capital letters, numbers, or punctuation
	// limited to these characters: “'._-” (apostrophe, period, underscore, dash.).
	return validCurrencyRegex.MatchString(t.Currency)
}
