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
	SourceType string
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
// Intentionaly doesn't check for IgnoreSubstrings.
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
	// 1) validate currencies (transaction may have 1 or 2 currencies),
	// 2) find all accounts and on which timespan it was used
	accounts := make(map[string]AccountFromTo)
	currencies := make(map[string]struct{})
	for _, t := range transactions {
		// Check account currency.
		atLeastOneCurrency := false
		if len(t.AccountCurrency) > 0 {
			if validCurrencyRegex.MatchString(t.AccountCurrency) {
				currencies[t.AccountCurrency] = struct{}{}
				atLeastOneCurrency = true
			} else {
				return 0, fmt.Errorf(
					"invalid currency '%s' in file '%s' from transaction: %+v",
					t.AccountCurrency, t.Source, t,
				)
			}
		}
		// Check origin currency.
		if t.OriginCurrency != "" {
			if validCurrencyRegex.MatchString(t.OriginCurrency) {
				currencies[t.OriginCurrency] = struct{}{}
				atLeastOneCurrency = true
			} else {
				return 0, fmt.Errorf(
					"invalid origin currency '%s' in file '%s' from transaction: %+v",
					t.OriginCurrency, t.Source, t,
				)
			}
		}
		if !atLeastOneCurrency {
			return 0, fmt.Errorf(
				"no currency found in transaction '%+v' from file '%s'",
				t, t.Source,
			)
		}
		updateAccounts(accounts, t.ToAccount, t.SourceType, t.Date, t.IsExpense)
		updateAccounts(accounts, t.FromAccount, t.SourceType, t.Date, t.IsExpense)
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
			"%s open Income:%s:%s\n",
			fromTo.From.Format(beancountOutputTimeFormat),
			fromTo.SourceType,
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
		// Add extra line and comment with transaction 'direction' and source file.
		name := "expense"
		if !t.IsExpense {
			name = "income"
		}
		sb.WriteString(fmt.Sprintf("\n; %s from %s '%s'\n", name, t.SourceType, t.Source))
		// 2014-05-05 * "Some details"
		sb.WriteString(fmt.Sprintf("%s * \"%s\"\n", t.Date.Format(beancountOutputTimeFormat), t.Details))
		// FYI: transactions may be provided in different currencies:
		// - origin currency only -> use it
		// - account currency only -> use it
		// - both account and origin currencies -> put origin currency and '@@' account currency.
		isAccountAmount := len(t.AccountCurrency) > 0 && t.Amount.int != 0
		isOriginCurAmount := len(t.OriginCurrency) > 0 && t.OriginCurrencyAmount.int != 0
		// If both currencies provided and are equal then use only "account" currency.
		if isAccountAmount && isOriginCurAmount && t.AccountCurrency == t.OriginCurrency {
			isOriginCurAmount = false
		}
		if t.IsExpense {
			source := fmt.Sprintf("Assets:%s:%s", t.SourceType, t.FromAccount)
			destination := fmt.Sprintf("Expenses:%s", *category)
			// Check destination account is my own account.
			if accountData, ok := accounts[t.ToAccount]; ok {
				if accountData.SourceType != "" {
					destination = fmt.Sprintf("Assets:%s:%s", accountData.SourceType, t.ToAccount)
				}
			}
			if isAccountAmount && isOriginCurAmount {
				// Assets:SourceType:AccountNumber  -100 USD @@ 40000 AMD
				// DESTINATION                       100 USD @@ 40000 AMD
				// (where DESTINATION is Expenses:Category or Assets:SourceType:AccountNumber)
				sb.WriteString(
					fmt.Sprintf("  %s    -%s %s @@ %s %s\n",
						source,
						t.Amount.StringNoIndent(),
						t.AccountCurrency,
						t.OriginCurrencyAmount.StringNoIndent(),
						t.OriginCurrency,
					),
				)
				sb.WriteString(
					fmt.Sprintf("  %s    %s %s @@ %s %s\n",
						destination,
						t.Amount.StringNoIndent(),
						t.AccountCurrency,
						t.OriginCurrencyAmount.StringNoIndent(),
						t.OriginCurrency,
					),
				)
			} else if isAccountAmount {
				sb.WriteString(
					fmt.Sprintf("  %s    -%s %s\n",
						source,
						t.Amount.StringNoIndent(),
						t.AccountCurrency,
					),
				)
				sb.WriteString(
					fmt.Sprintf("  %s    %s %s\n",
						destination,
						t.Amount.StringNoIndent(),
						t.AccountCurrency,
					),
				)

			} else if isOriginCurAmount {
				sb.WriteString(
					fmt.Sprintf("  %s    -%s %s\n",
						source,
						t.OriginCurrencyAmount.StringNoIndent(),
						t.OriginCurrency,
					),
				)
				sb.WriteString(
					fmt.Sprintf("  %s    %s %s\n",
						destination,
						t.OriginCurrencyAmount.StringNoIndent(),
						t.OriginCurrency,
					),
				)
			} else {
				return 0, fmt.Errorf(
					"transaction '%+v' has no amount in account or origin currency",
					t,
				)
			}
		} else { // Income
			source := fmt.Sprintf("Income:%s", t.FromAccount)
			// Check source account is my own account.
			if accountData, ok := accounts[t.FromAccount]; ok {
				if accountData.SourceType != "" {
					source = fmt.Sprintf("Assets:%s:%s", accountData.SourceType, t.FromAccount)
				}
			}
			destination := fmt.Sprintf("Assets:%s:%s", t.SourceType, t.ToAccount)
			if isAccountAmount && isOriginCurAmount {
				// SOURCE                            100 USD @@ 40000 AMD
				// Assets:SourceType:AccountNumber  -100 USD @@ 40000 AMD
				// (where SOURCE is Income:ForeignAccount or Assets:SourceType:AccountNumber)
				sb.WriteString(
					fmt.Sprintf("  %s    %s %s @@ %s %s\n",
						source,
						t.Amount.StringNoIndent(),
						t.AccountCurrency,
						t.OriginCurrencyAmount.StringNoIndent(),
						t.OriginCurrency,
					),
				)
				sb.WriteString(
					fmt.Sprintf("  %s    -%s %s @@ %s %s\n",
						destination,
						t.Amount.StringNoIndent(),
						t.AccountCurrency,
						t.OriginCurrencyAmount.StringNoIndent(),
						t.OriginCurrency,
					),
				)
			} else if isAccountAmount {
				sb.WriteString(
					fmt.Sprintf("  %s    %s %s\n",
						source,
						t.Amount.StringNoIndent(),
						t.AccountCurrency,
					),
				)
				sb.WriteString(
					fmt.Sprintf("  %s    -%s %s\n",
						destination,
						t.Amount.StringNoIndent(),
						t.AccountCurrency,
					),
				)

			} else if isOriginCurAmount {
				sb.WriteString(
					fmt.Sprintf("  %s    %s %s\n",
						source,
						t.OriginCurrencyAmount.StringNoIndent(),
						t.OriginCurrency,
					),
				)
				sb.WriteString(
					fmt.Sprintf("  %s    -%s %s\n",
						destination,
						t.OriginCurrencyAmount.StringNoIndent(),
						t.OriginCurrency,
					),
				)
			} else {
				return 0, fmt.Errorf(
					"transaction '%+v' has no amount in account or origin currency",
					t,
				)
			}
		}
		file.WriteString(sb.String())
	}

	return len(transactions), nil
}

func updateAccounts(accounts map[string]AccountFromTo, account string, sourceType string, date time.Time, isExpense bool) {
	if len(account) > 0 {
		if fromTo, ok := accounts[account]; !ok {
			accounts[account] = AccountFromTo{
				From: date,
				To:   date,
				SourceType: sourceType,
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
	return validCurrencyRegex.MatchString(t.AccountCurrency)
}

var validAccountNameRegex = regexp.MustCompile(`[^\p{L}\p{N}]+`)

func normalizeAccountName(account string) string {
	normalized := validAccountNameRegex.ReplaceAllString(account, "-")
	return strings.Trim(normalized, "-")
}
