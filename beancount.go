package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
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

const beancountOutputTimeFormat = "2006-01-02"

type AccountFromTo struct {
	From, To time.Time
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

// buildBeanconFile creates a beancount file with transactions.
// Returns number of transactions and error if any.
func buildBeanconFile(transactions []Transaction, config *Config, outputFileName string) (int, error) {

	// First check config
	// Invert GroupNamesToSubstrings and check for duplicates.
	substringsToGroupName := map[string]string{}
	for name, substrings := range config.GroupNamesToSubstrings {
		for _, substring := range substrings {
			if group, exist := substringsToGroupName[substring]; exist {
				return 0, fmt.Errorf("substring '%s' is duplicated in groups: '%s', '%s'",
					substring, name, group)
			}
			substringsToGroupName[substring] = name
		}
	}
	log.Printf("Beancount report: going to categorize transactions by %d named groups from %d substrings",
		len(config.GroupNamesToSubstrings), len(substringsToGroupName))

	// Sort transactions by date to simplify processing.
	sort.Sort(TransactionList(transactions))

	// First iterate all transactions to:
	// 1) validate currencies,
	// 2) find all accounts and on which timespan it was used
	accounts := make(map[string]AccountFromTo)
	currencies := make(map[string]struct{})
	for _, t := range transactions {
		if validCurrencyRegex.MatchString(t.Currency) {
			currencies[t.Currency] = struct{}{}
		} else {
			return 0, fmt.Errorf(
				"invalid currency '%s' in file '%s' from transaction: %+v",
				t.Currency, t.Source, t,
			)
		}
		updateAccounts(accounts, t.ToAccount, t.Date, t.IsExpense)
		updateAccounts(accounts, t.FromAccount, t.Date, t.IsExpense)
	}

	// Create accounts.beancount file.
	file, err := os.Create(outputFileName)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Setup plugins.
	// Don't create account for the each expense category.
	fmt.Fprintln(file, "plugin \"beancount.plugins.auto_accounts\"")
	fmt.Fprintln(file, "")

	// Dump "operating currencies".
	for currency := range currencies {
		fmt.Fprintf(file, "option \"operating_currency\" \"%s\"\n", currency)
	}
	fmt.Fprintln(file, "")

	// Dump "open accounts".
	fmt.Fprintln(file, ";; Open accounts")
	for account, fromTo := range accounts {
		fmt.Fprintf(
			file,
			"%s open Income:User:%s\n",
			fromTo.From.Format(beancountOutputTimeFormat),
			account,
		)
	}
	fmt.Fprintln(file, "")

	// Now iterate all transactions, find expenses category and dump.
	// Prepare "group name - substrings" map
	for _, t := range transactions {

		// First check that need to ignore transaction.
		for _, substring := range config.IgnoreSubstrings {
			if strings.Contains(t.Details, substring) {
				continue
			}
		}

		// Find category.
		var category *string = nil
		for substring, groupName := range substringsToGroupName {
			if strings.Contains(t.Details, substring) {
				category = &groupName
				break
			}
		}

		// Otherwise add transaction to either "unknown" or personal group.
		if category == nil {
			// Choose name of group to add transaction into.
			if config.GroupAllUnknownTransactions {
				unknownCategory := "unknown"
				category = &unknownCategory
			} else {
				category = &t.Details
			}
		}

		// Add transaction to the file.
		var sb strings.Builder
		// Make category name to be a valid account name.
		*category = normalizeAccountName(*category)
		// Add comment with source file.
		sb.WriteString(fmt.Sprintf("; isExpense=%t, source=%s\n", t.IsExpense, t.Source))
		// 2014-05-05 * "Some details"
		sb.WriteString(fmt.Sprintf("%s * \"%s\"\n", t.Date.Format(beancountOutputTimeFormat), t.Details))
		if t.IsExpense {
			// Income:MyAccount  -100 USD
			sb.WriteString(fmt.Sprintf("  Income:%s    -%s %s\n", t.FromAccount, t.Amount.StringNoIndent(), t.Currency))
			// Expense:Category  100 USD
			sb.WriteString(fmt.Sprintf("  Expenses:%s    %s %s\n", *category, t.Amount.StringNoIndent(), t.Currency))
		} else {
			// Income:MyAccount:Category  100 USD
			sb.WriteString(fmt.Sprintf("  Income:%s:%s    %s %s\n", t.ToAccount, *category, t.Amount.StringNoIndent(), t.Currency))
			if t.FromAccount != "" {
				// Assets:ForeignAccout  - 100 USD
				sb.WriteString(fmt.Sprintf("  Assets:%s    -%s %s\n", t.FromAccount, t.Amount.StringNoIndent(), t.Currency))
			}
		}
		file.WriteString(sb.String())
	}

	return len(transactions), nil
}

func updateAccounts(accounts map[string]AccountFromTo, account string, date time.Time, isExpense bool) {
	if len(account) > 0 {
		if fromTo, ok := accounts[account]; !ok {
			accounts[account] = AccountFromTo{
				From: date,
				To:   date,
			}
		} else {
			if isExpense {
				fromTo.To = date
			} else {
				fromTo.From = date
			}
		}
	}
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

var validAccountNameRegex = regexp.MustCompile(`[^\p{L}\p{N}]+`)

func normalizeAccountName(account string) string {
	normalized := validAccountNameRegex.ReplaceAllString(account, "-")
	return strings.Trim(normalized, "-")
}
