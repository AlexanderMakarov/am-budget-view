package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
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

// buildBeancountFile creates a beancount file with journal entries.
// Returns number of journal entries and error if any.
func buildBeancountFile(journalEntries []JournalEntry, currencies map[string]*CurrencyStatistics, accounts map[string]*AccountFromTransactions, outputFileName string) (int, error) {

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

	// Check all found accounts and dump "open accounts" for my own accounts.
	fmt.Fprintln(file, ";; Open accounts")
	for _, account := range accounts {
		if account.SourceType != "" {
			fmt.Fprintf(
				file,
				"%s open Assets:%s:%s\n",
				account.From.Format(beancountOutputTimeFormat),
				account.SourceType,
				account.Number,
			)
		}
	}
	fmt.Fprintln(file, "")

	// Now iterate all Journal Entries, find expenses category and dump.
	// Prepare "group name - substrings" map
	for _, je := range journalEntries {
		// Validate currencies.
		if je.AccountCurrency != "" && !checkCurrency(je.AccountCurrency) {
			return 0, errors.New(
				i18n.T("invalid account currency c in journal entry t",
					"c", je.AccountCurrency, "t", je,
				),
			)
		}
		if je.OriginCurrency != "" && !checkCurrency(je.OriginCurrency) {
			return 0, errors.New(
				i18n.T("invalid origin currency c in journal entry t",
					"c", je.OriginCurrency, "t", je,
				),
			)
		}
		// Add journal entry to the file.
		var sb strings.Builder
		// Make category name to be a valid account name.
		categoryName := normalizeAccountName(je.Category)
		// Add extra line and comment with transaction 'direction' and source file.
		name := "expense"
		if !je.IsExpense {
			name = "income"
		}
		sb.WriteString(fmt.Sprintf("\n; %s from %s '%s'\n", name, je.SourceType, je.Source))
		// 2014-05-05 * "Some details"
		sb.WriteString(fmt.Sprintf("%s * \"%s\"\n", je.Date.Format(beancountOutputTimeFormat), je.Details))
		// FYI: transaction (source of journal entry) may be provided in different currencies:
		// - origin currency only -> use it
		// - account currency only -> use it
		// - both account and origin currencies -> put origin currency and '@@' account currency.
		isAccountAmount := len(je.AccountCurrency) > 0 && je.AccountCurrencyAmount.int != 0
		isOriginCurAmount := len(je.OriginCurrency) > 0 && je.OriginCurrencyAmount.int != 0
		// If both currencies provided and are equal then use only "account" currency.
		if isAccountAmount && isOriginCurAmount && je.AccountCurrency == je.OriginCurrency {
			isOriginCurAmount = false
		}
		if je.IsExpense {
			source := ""
			if account, ok := accounts[je.FromAccount]; ok {
				source = fmt.Sprintf("Assets:%s:%s", account.SourceType, account.Number)
			} else {
				// Expense from unknown account should not happen.
				return 0, errors.New(
					i18n.T("source account a not found",
						"a", je.FromAccount,
					),
				)
			}
			destination := ""
			if account, ok := accounts[je.ToAccount]; ok && account.SourceType != "" {
				destination = fmt.Sprintf("Expenses:%s:%s", account.SourceType, account.Number)
			} else {
				destination = fmt.Sprintf("Expenses:%s:%s", categoryName, je.ToAccount)
			}
			if isAccountAmount && isOriginCurAmount {
				// SOURCE        -100 USD @@ 40000 AMD
				// DESTINATION  100 USD @@ 40000 AMD
				sb.WriteString(
					fmt.Sprintf("  %s    -%s %s @@ %s %s\n",
						source,
						je.AccountCurrencyAmount.StringNoIndent(),
						je.AccountCurrency,
						je.OriginCurrencyAmount.StringNoIndent(),
						je.OriginCurrency,
					),
				)
				sb.WriteString(
					fmt.Sprintf("  %s    %s %s @@ %s %s\n",
						destination,
						je.AccountCurrencyAmount.StringNoIndent(),
						je.AccountCurrency,
						je.OriginCurrencyAmount.StringNoIndent(),
						je.OriginCurrency,
					),
				)
			} else if isAccountAmount {
				sb.WriteString(
					fmt.Sprintf("  %s    -%s %s\n",
						source,
						je.AccountCurrencyAmount.StringNoIndent(),
						je.AccountCurrency,
					),
				)
				sb.WriteString(
					fmt.Sprintf("  %s    %s %s\n",
						destination,
						je.AccountCurrencyAmount.StringNoIndent(),
						je.AccountCurrency,
					),
				)

			} else if isOriginCurAmount {
				sb.WriteString(
					fmt.Sprintf("  %s    -%s %s\n",
						source,
						je.OriginCurrencyAmount.StringNoIndent(),
						je.OriginCurrency,
					),
				)
				sb.WriteString(
					fmt.Sprintf("  %s    %s %s\n",
						destination,
						je.OriginCurrencyAmount.StringNoIndent(),
						je.OriginCurrency,
					),
				)
			} else {
				return 0, errors.New(
					i18n.T("journal entry t has no amount in account or origin currency",
						"t", je,
					),
				)
			}
		} else { // Income
			source := ""
			if account, ok := accounts[je.FromAccount]; ok && account.SourceType != "" {
				source = fmt.Sprintf("Income:%s:%s", account.SourceType, account.Number)
			} else {
				source = fmt.Sprintf("Income:%s:%s", categoryName, je.FromAccount)
			}
			destination := ""
			if account, ok := accounts[je.ToAccount]; ok {
				destination = fmt.Sprintf("Assets:%s:%s", account.SourceType, account.Number)
			} else {
				// Income to unknown account should not happen.
				return 0, errors.New(
					i18n.T("destination account a not found",
						"a", je.ToAccount,
					),
				)
			}
			if isAccountAmount && isOriginCurAmount {
				// SOURCE       -100 USD @@ 40000 AMD
				// DESTINATION  100 USD @@ 40000 AMD
				sb.WriteString(
					fmt.Sprintf("  %s    -%s %s @@ %s %s\n",
						source,
						je.AccountCurrencyAmount.StringNoIndent(),
						je.AccountCurrency,
						je.OriginCurrencyAmount.StringNoIndent(),
						je.OriginCurrency,
					),
				)
				sb.WriteString(
					fmt.Sprintf("  %s    %s %s @@ %s %s\n",
						destination,
						je.AccountCurrencyAmount.StringNoIndent(),
						je.AccountCurrency,
						je.OriginCurrencyAmount.StringNoIndent(),
						je.OriginCurrency,
					),
				)
			} else if isAccountAmount {
				sb.WriteString(
					fmt.Sprintf("  %s    -%s %s\n",
						source,
						je.AccountCurrencyAmount.StringNoIndent(),
						je.AccountCurrency,
					),
				)
				sb.WriteString(
					fmt.Sprintf("  %s    %s %s\n",
						destination,
						je.AccountCurrencyAmount.StringNoIndent(),
						je.AccountCurrency,
					),
				)

			} else if isOriginCurAmount {
				sb.WriteString(
					fmt.Sprintf("  %s    -%s %s\n",
						source,
						je.OriginCurrencyAmount.StringNoIndent(),
						je.OriginCurrency,
					),
				)
				sb.WriteString(
					fmt.Sprintf("  %s    %s %s\n",
						destination,
						je.OriginCurrencyAmount.StringNoIndent(),
						je.OriginCurrency,
					),
				)
			} else {
				return 0, errors.New(
					i18n.T("journal entry t has no amount in account or origin currency",
						"t", je,
					),
				)
			}
		}
		file.WriteString(sb.String())
	}

	return len(journalEntries), nil
}

var validCurrencyRegex = regexp.MustCompile(`^[A-Z][A-Z0-9'._-]{0,22}[A-Z0-9]$`)

func checkCurrency(currency string) bool {
	// Currency should be uppercased up to 24 characters,
	// and it must start with a capital letter,
	// must end with with a capital letter or number,
	// its other characters must only be capital letters, numbers, or punctuation
	// limited to these characters: “'._-” (apostrophe, period, underscore, dash.).
	return validCurrencyRegex.MatchString(currency)
}

var validAccountNameRegex = regexp.MustCompile(`[^\p{L}\p{N}]+`)

func normalizeAccountName(account string) string {
	normalized := validAccountNameRegex.ReplaceAllString(account, "-")
	return strings.Trim(normalized, "-")
}

func printCurrencyStatisticsMap(convertableCurrencies map[string]*CurrencyStatistics) {
	if len(convertableCurrencies) == 0 {
		fmt.Println(i18n.T("No currencies found"))
		return
	}
	fmt.Println(i18n.T("Currency\tFrom\tTo\tNumber of Exchange Rates"))
	for currency, stat := range convertableCurrencies {
		fmt.Printf("  %s\t%s\t%s\t%d\n",
			currency,
			stat.From.Format(beancountOutputTimeFormat),
			stat.To.Format(beancountOutputTimeFormat),
			len(stat.ExchangeRates))
	}
}
