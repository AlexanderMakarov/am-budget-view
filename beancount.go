package main

import (
	"fmt"
	"log"
	"math"
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

func (m MoneyWith2DecimalPlaces) StringNoIndent() string {
	dollars := m.int / 100
	cents := m.int % 100
	dollarString := strconv.Itoa(dollars)
	for i := len(dollarString) - 3; i > 0; i -= 3 {
		dollarString = dollarString[:i] + "," + dollarString[i:]
	}
	return fmt.Sprintf("%s.%02d", dollarString, cents)
}

type ExchangeRate struct {
	// Date is a date of the exchange rate.
	date time.Time
	// CurrencyFrom is a always account currency.
	currencyFrom string
	// CurrencyTo is a always origin currency.
	currencyTo string
	// ExchangeRate = CurrencyFrom / CurrencyTo.
	exchangeRate float64
}

type CurrencyStatistics struct {
	// Name is a currency name (valid by beancount rules).
	Name string
	// From is a first transaction date.
	From time.Time
	// To is a last transaction date.
	To time.Time
	// MetInSources is a set of sources where currency was met.
	MetInSources map[string]struct{}
	// MetTimes is a number of times currency was met.
	MetTimes int
	// OverlappedWithOtherCurrencyAmount is a total amount of the currency overlapped with other currency.
	OverlappedWithOtherCurrencyAmount MoneyWith2DecimalPlaces
	// TotalAmount is a total amount of the currency.
	TotalAmount MoneyWith2DecimalPlaces
	// Transactions is a list of transactions with the currency.
	Transactions []*Transaction
	// ExchangeRates is a list of exchange rates for the currency.
	// Current currency may be both "currencyFrom" and "currencyTo" in exchange rates.
	ExchangeRates []*ExchangeRate
}

type currencyState struct {
	currency          string
	statistics        *CurrencyStatistics
	lastExchangeRate  *ExchangeRate
	exchangeRateIndex int
}

func findClosestExchangeRate(
	date time.Time,
	curState *currencyState,
) (*ExchangeRate, int, int) {
	exchangeRate := curState.lastExchangeRate
	exchangeRateIndex := curState.exchangeRateIndex
	dateDiff := date.Sub(exchangeRate.date)
	for i := exchangeRateIndex + 1; i < len(curState.statistics.ExchangeRates); i++ {
		checkedEr := curState.statistics.ExchangeRates[i]
		if date.Sub(checkedEr.date).Abs() < dateDiff.Abs() {
			exchangeRate = checkedEr
			dateDiff = date.Sub(checkedEr.date)
			exchangeRateIndex = i
		}
	}
	return exchangeRate, exchangeRateIndex, int(dateDiff / (24 * time.Hour))
}

func findClosestExchangeRateToCurrency(
	date time.Time,
	currency string,
	curState *currencyState,
) (*ExchangeRate, int, int) {
	var exchangeRate *ExchangeRate = nil
	exchangeRateIndex := curState.exchangeRateIndex
	var dateDiff time.Duration = time.Duration(math.MaxInt64)
	for i := exchangeRateIndex; i < len(curState.statistics.ExchangeRates); i++ {
		checkedEr := curState.statistics.ExchangeRates[i]
		if checkedEr.currencyTo != currency && checkedEr.currencyFrom != currency {
			continue
		}
		if checkedEr.date.Sub(date).Abs() < dateDiff.Abs() {
			exchangeRate = checkedEr
			dateDiff = date.Sub(checkedEr.date)
			exchangeRateIndex = i
		}
	}
	return exchangeRate, exchangeRateIndex, int(dateDiff.Abs() / (24 * time.Hour))
}

// convertToCurrency converts transaction amounts to convertable currencies.
// Shifts index of exchangeRateIndex in curStates because expects to be called for dates in chronological order.
// Returns converted amount and number representing how precise conversion was.
// 0 - no conversion (amountCurrency == targetCurrency),
// 1 - with direct exchange rate to targetCurrency at the same date,
// >1 - number of days between transaction date to used exchange rate date, plus the number of days to the next exchange rate if first one was not direct.
func convertToCurrency(
	amount MoneyWith2DecimalPlaces,
	amountCurrency string,
	targetCurrency string,
	date time.Time,
	curStates map[string]currencyState,
) (MoneyWith2DecimalPlaces, int) {
	// If the same currency then no conversion, precision is 0.
	if amountCurrency == targetCurrency {
		return amount, 0
	}

	// Try to find direct exchange rate.
	curState := curStates[amountCurrency]
	exchangeRateDirect, exchangeRateIndexDirect, daysDiffDirect := findClosestExchangeRateToCurrency(date, targetCurrency, &curState)
	if exchangeRateDirect != nil {
		curState.lastExchangeRate = exchangeRateDirect
		curState.exchangeRateIndex = exchangeRateIndexDirect
		precision := daysDiffDirect
		// If exchange rate is for the same date then set precision to 1, otherwise keep it as days difference.
		if precision == 0 {
			precision = 1
		}
		if exchangeRateDirect.currencyTo == amountCurrency {
			return MoneyWith2DecimalPlaces{int: int(float64(amount.int) * exchangeRateDirect.exchangeRate)}, precision
		}
		return MoneyWith2DecimalPlaces{int: int(float64(amount.int) / exchangeRateDirect.exchangeRate)}, precision
	}

	// Otherwise try to find exchange rate by multiple conversions.
	// Use Bellman-Ford algorithm to find shortest path (with minimal precision loss).
	// Don't advance exchange rate index in curStates here.
	// Generated by Claude 3.5 20241022.

	// Find all closest to date exchange rates to all currencies.
	type currencyNode struct {
		currency  string
		amount    MoneyWith2DecimalPlaces
		precision int
	}

	// Build graph of exchange rates between currencies.
	nodes := make(map[string]*currencyNode)
	// Initialize nodes with infinity amounts except source currency.
	for currency := range curStates {
		nodes[currency] = &currencyNode{
			currency:  currency,
			amount:    MoneyWith2DecimalPlaces{int: math.MaxInt},
			precision: math.MaxInt,
		}
	}
	// Set initial amount for source currency.
	nodes[amountCurrency] = &currencyNode{
		currency:  amountCurrency,
		amount:    amount,
		precision: 0,
	}
	// Run Bellman-Ford algorithm to find shortest path (with minimal precision loss).
	// Need V-1 iterations where V is number of currencies.
	for i := 0; i < len(nodes)-1; i++ {
		// For each currency try to find better conversion through its exchange rates.
		for _, curState := range curStates {
			// Skip currencies that we can't reach yet.
			fromNode, exists := nodes[curState.currency]
			if !exists || fromNode.amount.int == math.MaxInt {
				continue
			}
			// Try each exchange rate from this currency.
			for _, er := range curState.statistics.ExchangeRates {
				// Find closest exchange rate to date.
				_, _, daysDiff := findClosestExchangeRate(date, &curState)
				// Calculate new precision - days difference plus current node precision.
				newPrecision := daysDiff
				if newPrecision == 0 {
					newPrecision = 1
				}
				newPrecision += fromNode.precision
				// Calculate converted amount.
				var newAmount MoneyWith2DecimalPlaces
				toCurrency := er.currencyTo
				if er.currencyTo == curState.currency {
					toCurrency = er.currencyFrom
					newAmount = MoneyWith2DecimalPlaces{
						int: int(float64(fromNode.amount.int) / er.exchangeRate),
					}
				} else {
					newAmount = MoneyWith2DecimalPlaces{
						int: int(float64(fromNode.amount.int) * er.exchangeRate),
					}
				}
				// Update target node if found better precision.
				toNode, exists := nodes[toCurrency]
				if !exists {
					continue
				}
				if newPrecision < toNode.precision {
					toNode.amount = newAmount
					toNode.precision = newPrecision
				}
			}
		}
	}
	// Return converted amount and precision for target currency.
	if targetNode, exists := nodes[targetCurrency]; exists {
		if targetNode.amount.int != math.MaxInt {
			return targetNode.amount, targetNode.precision
		}
	}
	// If no conversion path found then return original amount with max precision.
	return amount, math.MaxInt
}

func buildJournalEntries(
	transactions []Transaction,
	config *Config,
) (
	[]JournalEntry,
	map[string]*AccountFromTransactions,
	map[string]*CurrencyStatistics,
	error,
) {

	// First check config
	// Invert GroupNamesToSubstrings and check for duplicates.
	substringsToGroupName := map[string]string{}
	for name, substrings := range config.GroupNamesToSubstrings {
		for _, substring := range substrings {
			if group, exist := substringsToGroupName[substring]; exist {
				return nil, nil, nil, fmt.Errorf("substring '%s' is duplicated in groups: '%s', '%s'",
					substring, name, group)
			}
			substringsToGroupName[substring] = name
		}
	}
	log.Printf("Going to categorize transactions by %d named groups from %d substrings",
		len(config.GroupNamesToSubstrings), len(substringsToGroupName))

	// Sort transactions by date to simplify processing.
	sort.Sort(TransactionList(transactions))

	// Iterate all transactions to:
	// 1) validate and collect currencies (transaction may have 1 or 2 currencies), determine their timespan
	// 2) find all accounts, detemine theirs type, timespan
	// 3) make list of available exchange rates
	accounts := make(map[string]*AccountFromTransactions)
	currencies := make(map[string]*CurrencyStatistics)
	for _, t := range transactions {
		var exchangeRate *ExchangeRate = nil
		// Check account currency.
		atLeastOneCurrency := false
		if len(t.AccountCurrency) > 0 {
			if validCurrencyRegex.MatchString(t.AccountCurrency) {
				currency, ok := currencies[t.AccountCurrency]
				if !ok {
					currency = &CurrencyStatistics{
						Name:          t.AccountCurrency,
						From:          t.Date,
						MetInSources:  make(map[string]struct{}),
						Transactions:  []*Transaction{&t},
						ExchangeRates: []*ExchangeRate{},
					}
					currencies[t.AccountCurrency] = currency
				}
				currency.MetInSources[t.SourceType] = struct{}{}
				currency.MetTimes++
				currency.To = t.Date
				if t.OriginCurrency != "" {
					currency.OverlappedWithOtherCurrencyAmount.int += t.Amount.int
					// If transaction has both currencies amounts then add exchange rate to the list.
					// Do it only once per transaction (check for OriginCurrency validity would be later).
					if t.Amount.int != 0 && t.OriginCurrencyAmount.int != 0 {
						exchangeRate = &ExchangeRate{
							date:         t.Date,
							currencyFrom: t.AccountCurrency,
							currencyTo:   t.OriginCurrency,
							exchangeRate: float64(t.Amount.int) / float64(t.OriginCurrencyAmount.int),
						}
						currency.ExchangeRates = append(currency.ExchangeRates, exchangeRate)
					}
				}
				currency.TotalAmount.int += t.Amount.int
				atLeastOneCurrency = true
			} else {
				return nil, nil, nil, fmt.Errorf(
					"invalid currency '%s' in file '%s' from transaction: %+v",
					t.AccountCurrency, t.Source, t,
				)
			}
		}
		// Check origin currency.
		if t.OriginCurrency != "" {
			if validCurrencyRegex.MatchString(t.OriginCurrency) {
				// If transaction has both currencies then they should be different.
				if atLeastOneCurrency && t.OriginCurrency == t.AccountCurrency {
					return nil, nil, nil, fmt.Errorf(
						"transaction '%+v' has the same currencies in account and origin: '%s', '%s'",
						t, t.AccountCurrency, t.OriginCurrency,
					)
				}
				currency, ok := currencies[t.OriginCurrency]
				if !ok {
					currency = &CurrencyStatistics{
						Name:          t.OriginCurrency,
						From:          t.Date,
						MetInSources:  make(map[string]struct{}),
						Transactions:  []*Transaction{&t},
						ExchangeRates: []*ExchangeRate{},
					}
					currencies[t.OriginCurrency] = currency
				}
				currency.MetInSources[t.SourceType] = struct{}{}
				currency.MetTimes++
				currency.To = t.Date
				if t.AccountCurrency != "" {
					currency.OverlappedWithOtherCurrencyAmount.int += t.Amount.int
				}
				if exchangeRate != nil {
					currency.ExchangeRates = append(currency.ExchangeRates, exchangeRate)
				}
				currency.TotalAmount.int += t.OriginCurrencyAmount.int
				atLeastOneCurrency = true
			} else {
				return nil, nil, nil, fmt.Errorf(
					"invalid origin currency '%s' in file '%s' from transaction: %+v",
					t.OriginCurrency, t.Source, t,
				)
			}
		}
		// Check that transaction has at least one currency.
		if !atLeastOneCurrency {
			return nil, nil, nil, fmt.Errorf(
				"no currency found in transaction '%+v' from file '%s'",
				t, t.Source,
			)
		}
		// Handle destination account.
		if len(t.ToAccount) > 0 {
			if account, ok := accounts[t.ToAccount]; !ok {
				sourceType := ""
				// Income transaction's "ToAccount" is my own account.
				if !t.IsExpense && len(t.SourceType) > 0 {
					sourceType = t.SourceType
				}
				accounts[t.ToAccount] = &AccountFromTransactions{
					IsTransactionAccount: !t.IsExpense,
					SourceType:           sourceType,
					Source:               t.Source,
					From:                 t.Date,
					To:                   t.Date,
					Number:               t.ToAccount,
				}
			} else {
				// Expect transactions are sorted by date.
				account.To = t.Date
				if !t.IsExpense && len(t.SourceType) > 0 {
					account.SourceType = t.SourceType
					account.Source = t.Source
				}
				if !t.IsExpense {
					account.IsTransactionAccount = true
				}
			}
		}
		// Handle source account.
		if len(t.FromAccount) > 0 {
			if account, ok := accounts[t.FromAccount]; !ok {
				sourceType := ""
				// Expense transaction's "FromAccount" is my own account.
				if t.IsExpense && len(t.SourceType) > 0 {
					sourceType = t.SourceType
				}
				accounts[t.FromAccount] = &AccountFromTransactions{
					IsTransactionAccount: t.IsExpense,
					SourceType:           sourceType,
					Source:               t.Source,
					From:                 t.Date,
					To:                   t.Date,
					Number:               t.FromAccount,
				}
			} else {
				// Expect transactions are sorted by date.
				account.To = t.Date
				if t.IsExpense && len(t.SourceType) > 0 {
					account.SourceType = t.SourceType
					account.Source = t.Source
				}
				if t.IsExpense {
					account.IsTransactionAccount = true
				}
			}
		}
	}
	if len(accounts) == 0 {
		return nil, nil, nil, fmt.Errorf("no accounts found")
	}
	if len(currencies) == 0 {
		return nil, nil, nil, fmt.Errorf("no currencies found")
	}
	fmt.Printf("In %d transactions found %d currencies:\n", len(transactions), len(currencies))
	printCurrencyStatisticsMap(currencies)

	// Find total timespan of all currencies.
	minDate := time.Time{}
	maxDate := time.Time{}
	for _, currency := range currencies {
		// Set initial values.
		if minDate.IsZero() {
			minDate = currency.From
		}
		if maxDate.IsZero() {
			maxDate = currency.To
		}
		// Update min and max dates if needed.
		if currency.From.Before(minDate) {
			minDate = currency.From
		}
		if currency.To.After(maxDate) {
			maxDate = currency.To
		}
	}
	totalTimespan := maxDate.Sub(minDate)
	fmt.Printf(
		"Transactions timespan: %s..%s (~%d months and %d days)\n",
		minDate.Format(beancountOutputTimeFormat),
		maxDate.Format(beancountOutputTimeFormat),
		int(totalTimespan.Hours()/24/30),
		int(totalTimespan.Hours()/24)%30,
	)

	// Determine in which currencies it makes sense to convert amounts in journal entries.
	// 1. Currency should span at least MinCurrencyTimespanPercent of total timespan.
	// 2. Currency should have no gaps longer than MaxCurrencyTimespanGapsDays.
	// Set defaults if config doesn't provide them.
	minTimespanPercent := config.MinCurrencyTimespanPercent
	if minTimespanPercent == 0 {
		minTimespanPercent = 80
	}
	maxGapDays := config.MaxCurrencyTimespanGapsDays
	if maxGapDays == 0 {
		maxGapDays = 30
	}
	// Calculate minTimespan and maxGap.
	minTimespan := time.Duration(minTimespanPercent) * totalTimespan / 100
	maxGap := time.Duration(maxGapDays) * 24 * time.Hour
	// Iterate all currencies to find convertable ones.
	// 1. Check timespan and gaps in exchange rates for any currency.
	convertableCurrencies := map[string]*CurrencyStatistics{}
	for _, stat := range currencies {
		// Check timespan.
		if stat.To.Sub(stat.From) < minTimespan {
			fmt.Printf("Currency '%s' has timespan %s which is less than minTimespan %s\n", stat.Name, stat.To.Sub(stat.From), minTimespan)
			continue
		}
		// Check that there are no gaps longer than maxGap for transactions with exchange rates.
		hasGapAtLeastDays := maxGap
		lastTransactionDate := minDate
		for _, er := range stat.ExchangeRates { // May be empty.
			hasGapAtLeastDays = er.date.Sub(lastTransactionDate)
			if hasGapAtLeastDays > maxGap {
				break
			}
			lastTransactionDate = er.date
		}
		if hasGapAtLeastDays > maxGap {
			fmt.Printf("Currency '%s' has gap in 'any' exchange rates %s which is longer than maxGap %s\n", stat.Name, hasGapAtLeastDays, maxGap)
			continue
		}
		convertableCurrencies[stat.Name] = stat
	}
	// 2. Iterate each currency exchange rates with checks they are for convertable currencies.
	// Do it until there are no gaps longer than maxGap for exchange rates.
	isRecheck := true
	for isRecheck {
		isRecheck = false
		for _, stat := range convertableCurrencies {
			hasGapAtLeastDays := maxGap
			lastTransactionDate := minDate
			for _, er := range stat.ExchangeRates {
				oppositeCurrency := er.currencyFrom
				if oppositeCurrency == stat.Name {
					oppositeCurrency = er.currencyTo
				}
				// Check that opposite currency is convertable.
				if _, ok := convertableCurrencies[oppositeCurrency]; !ok {
					continue
				}
				// Check that there are no gaps longer than maxGap for exchange rates.
				hasGapAtLeastDays = er.date.Sub(lastTransactionDate)
				if hasGapAtLeastDays >= maxGap {
					break
				}
				lastTransactionDate = er.date
			}
			// If there is a gap longer than maxGap then remove currency from the map.
			if hasGapAtLeastDays >= maxGap {
				fmt.Printf("Currency '%s' has gap in 'to convertible currencies' exchange rates %s which is longer than maxGap %s\n", stat.Name, hasGapAtLeastDays, maxGap)
				delete(convertableCurrencies, stat.Name)
				// Need to recheck all currencies all exchange rates one more time.
				isRecheck = true
				break
			}
			if isRecheck {
				break
			}
		}
	}
	fmt.Printf(
		"With MinCurrencyTimespanPercent=%d, MaxCurrencyTimespanGapsDays=%d filtered out following currencies to convert all journal entries amounts into:\n",
		minTimespanPercent,
		maxGapDays,
	)
	printCurrencyStatisticsMap(convertableCurrencies)

	// Append ConvertToCurrencies without any checks.
	for _, currency := range config.ConvertToCurrencies {
		if _, ok := currencies[currency]; !ok {
			return nil, nil, nil, fmt.Errorf("currency '%s' from ConvertToCurrencies not found in transactions", currency)
		}
		convertableCurrencies[currency] = currencies[currency]
	}

	// Check that we end up with at least one convertable currency.
	if len(convertableCurrencies) == 0 {
		return nil, nil, nil, fmt.Errorf("no convertable currencies found, consider a) force add ConvertToCurrencies (but with inprecise exchange rates), b) to decrease MinCurrencyTimespanPercent, c) to increase MaxCurrencyTimespanGapsDays")
	}

	// Make list of currencies to convert amounts into.
	curStates := make(map[string]currencyState, len(convertableCurrencies))
	for currency := range convertableCurrencies {
		statistics := currencies[currency]
		curStates[currency] = currencyState{
			currency:          currency,
			statistics:        statistics,
			lastExchangeRate:  statistics.ExchangeRates[0],
			exchangeRateIndex: 0,
		}
	}

	// Build journal entries.
	journalEntries := []JournalEntry{}
	for _, t := range transactions {
		// Try to find category.
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
		// Convert amounts to convertable currencies.
		amounts := make(map[string]AmountInCurrency, len(curStates))
		for _, state := range curStates {
			var amount1, amount2 MoneyWith2DecimalPlaces
			var precision1, precision2 int = math.MaxInt, math.MaxInt

			// Only convert if currency exists, in convertable currencies list, and amount is non-zero
			if t.AccountCurrency != "" && t.Amount.int != 0 {
				if _, exists := curStates[t.AccountCurrency]; exists {
					amount1, precision1 = convertToCurrency(t.Amount, t.AccountCurrency, state.currency, t.Date, curStates)
				}
			}

			// Only convert if currency exists, in convertable currencies list, and amount is non-zero
			if t.OriginCurrency != "" && t.OriginCurrencyAmount.int != 0 {
				if _, exists := curStates[t.OriginCurrency]; exists {
					amount2, precision2 = convertToCurrency(t.OriginCurrencyAmount, t.OriginCurrency, state.currency, t.Date, curStates)
				}
			}

			// Use the conversion with better precision
			if precision1 <= precision2 {
				amounts[state.currency] = AmountInCurrency{
					Currency:            state.currency,
					Amount:              amount1,
					ConversionPrecision: precision1,
				}
			} else {
				amounts[state.currency] = AmountInCurrency{
					Currency:            state.currency,
					Amount:              amount2,
					ConversionPrecision: precision2,
				}
			}
		}
		entry := JournalEntry{
			Date:                  t.Date,
			IsExpense:             t.IsExpense,
			SourceType:            t.SourceType,
			Source:                t.Source,
			Details:               t.Details,
			Category:              *category,
			AccountCurrency:       t.AccountCurrency,
			AccountCurrencyAmount: t.Amount,
			OriginCurrency:        t.OriginCurrency,
			OriginCurrencyAmount:  t.OriginCurrencyAmount,
			FromAccount:           t.FromAccount,
			ToAccount:             t.ToAccount,
			Amounts:               amounts,
		}
		journalEntries = append(journalEntries, entry)
	}

	fmt.Printf("Total assembled %d journal entries with amounts in %d currencies.\n", len(journalEntries), len(curStates))
	return journalEntries, accounts, currencies, nil
}

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
			return 0, fmt.Errorf("invalid account currency '%s' in journal entry '%+v'", je.AccountCurrency, je)
		}
		if je.OriginCurrency != "" && !checkCurrency(je.OriginCurrency) {
			return 0, fmt.Errorf("invalid origin currency '%s' in journal entry '%+v'", je.OriginCurrency, je)
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
				source = account.Number
			} else {
				// Expense from unknown account should not happen.
				return 0, fmt.Errorf("source account '%s' not found", je.FromAccount)
			}
			destination := ""
			if account, ok := accounts[je.ToAccount]; ok {
				destination = account.Number
			}
			// If account wasn't found or doesn't have a name then it is an expense to unknown account.
			if len(destination) == 0 {
				destination = fmt.Sprintf("Expenses:%s:%s", je.ToAccount, categoryName)
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
				return 0, fmt.Errorf(
					"journal entry '%+v' has no amount in account or origin currency",
					je,
				)
			}
		} else { // Income
			source := ""
			if account, ok := accounts[je.FromAccount]; ok {
				source = account.Number
			}
			// If account wasn't found or doesn't have a name then it is an income from unknown account.
			if len(source) == 0 {
				source = fmt.Sprintf("Income:%s:%s", je.FromAccount, categoryName)
			}
			destination := ""
			if account, ok := accounts[je.ToAccount]; ok {
				destination = account.Number
			} else {
				// Income to unknown account should not happen.
				return 0, fmt.Errorf("destination account '%s' not found", je.ToAccount)
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
				return 0, fmt.Errorf(
					"journal entry '%+v' has no amount in account or origin currency",
					je,
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
		fmt.Println("No currencies found.")
		return
	}
	fmt.Println("Currency\tFrom\tTo\t Number of Exchange Rates")
	for currency, stat := range convertableCurrencies {
		fmt.Printf("  %s\t%s\t%s\t%d\n",
			currency,
			stat.From.Format(beancountOutputTimeFormat),
			stat.To.Format(beancountOutputTimeFormat),
			len(stat.ExchangeRates))
	}
}
