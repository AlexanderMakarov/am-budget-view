package main

import (
	"errors"
	"fmt"
	"log"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ExchangeRate is a struct representing an exchange rate.
type ExchangeRate struct {
	// Date is a date of the exchange rate.
	date time.Time
	// CurrencyFrom is source currency name.
	// To get amount in source currency divide amount in target currency by rate value.
	currencyFrom string
	// CurrencyTo is target currency name.
	// To get amount in target currency multiply amount in source currency by rate value.
	currencyTo string
	// ExchangeRate = Amount in currencyFrom / Amount in currencyTo.
	exchangeRate float64
	// Source is a source of the exchange rate.
	source *TransactionsSource
}

func (er *ExchangeRate) String() string {
	return fmt.Sprintf("ExchangeRate{date: %v, currencyFrom: %q, currencyTo: %q, exchangeRate: %v, source: %+v}", er.date, er.currencyFrom, er.currencyTo, er.exchangeRate, er.source)
}

func atLeast1CentDiv(amountCents int, rate float64) MoneyWith2DecimalPlaces {
	n := int(float64(amountCents) / rate)
	if n == 0 {
		n = 1
	}
	return MoneyWith2DecimalPlaces{int: n}
}

func atLeast1CentMul(amountCents int, rate float64) MoneyWith2DecimalPlaces {
	n := int(float64(amountCents) * rate)
	if n == 0 {
		n = 1
	}
	return MoneyWith2DecimalPlaces{int: n}
}

// CurrencyStatistics is a struct representing data about a currency found in transactions.
type CurrencyStatistics struct {
	// Name is a currency name (valid by Beancount rules).
	Name string
	// From is a first transaction date.
	From time.Time
	// To is a last transaction date.
	To time.Time
	// MetInSources is a set of sources where currency was met.
	MetInSources map[string]struct{}
	// MetTimes is a number of times currency was occurred.
	MetTimes int
	// OverlappedWithOtherCurrencyAmount is a total amount of the currency overlapped with other currency.
	OverlappedWithOtherCurrencyAmount MoneyWith2DecimalPlaces
	// TotalAmount is a total amount of the currency.
	TotalAmount MoneyWith2DecimalPlaces
	// Transactions is a list of transactions with the currency.
	Transactions []*Transaction
	// ExchangeRates is a list of exchange rates for the currency.
	// Note that current currency could be both "currencyFrom" and "currencyTo" in exchange rates.
	ExchangeRates []*ExchangeRate
}

// DataMart is a container for transactions, accounts, currencies and exchange rates parsed from input files.
// Can be used to build journal entries.
type DataMart struct {
	// SortedTransactions is a sorted transactions list.
	SortedTransactions []Transaction
	// Accounts is a map of accounts statistics.
	Accounts map[string]*AccountStatistics
	// AllCurrencies is a map of all currencies statistics.
	AllCurrencies map[string]*CurrencyStatistics
	// ConvertibleCurrencies is a map of currencies for which conversion is possible.
	ConvertibleCurrencies map[string]*CurrencyStatistics
}

// currencyState contains data about a currency during one pass over transactions.
// Have to be recreated for each pass over transactions.
type currencyState struct {
	currency                       string
	statistics                     *CurrencyStatistics
	exchangeRateIndexesPerCurrency map[string]int
}

// findAmountNearCurrency searches a number in a string before specified index.
// Returns amount as integer with 2 decimal places.
func findAmountNearCurrency(details string, currencyIndex int) int {
	// Search for a number before specified index. Skip first space.
	amount := ""
	for i := currencyIndex - 2; i >= 0; i-- {
		rune := details[i]
		// Add all numbers and dots. Skip commas.
		if rune >= '0' && rune <= '9' || rune == '.' {
			amount = string(rune) + amount
		} else if rune == ',' {
			// Skip commas.
			continue
		} else if rune == ' ' && len(amount) == 0 {
			// Skip any number of spaces at the beginning.
			continue
		} else {
			break
		}
	}
	// If no amount found then return 0.
	if amount == "" {
		return 0
	}
	// Parse amount as float with 2 decimal places.
	amountFloat, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return 0
	}
	return int(amountFloat * 100)
}

// currencyRegex is a regex to find 3 upper case letters string with space before it.
var currencyRegex = regexp.MustCompile(`\s[A-Z]{3}\W`)

// parseExchangeRateFromDetails tries to parse exchange rate from details.
// Transaction details may contain it in formats
// - "330000 AMD / 4.4 = 75000 RUB"
// - "1550 EUR * 410.84 = 636802 AMD"
// - "1085500 AMD / 417.5 = 2600 EUR"
func parseExchangeRateFromDetails(date time.Time, details string, targetCurrency1, targetCurrency2 string, source *TransactionsSource) *ExchangeRate {
	// Try to find both currencies in details.
	currency1Index := strings.Index(details, targetCurrency1)
	currency2Index := strings.Index(details, targetCurrency2)
	// Check that at least one currency is present.
	// Note that index can't be 0 because amount should be placed before currency.
	currency1Found := currency1Index > 0
	currency2Found := currency2Index > 0
	if (currency1Found && !currency2Found) || (!currency1Found && currency2Found) {
		// Set currency to skip.
		skipCurrencyIndex := currency1Index
		if !currency1Found {
			skipCurrencyIndex = currency2Index
		}
		// Try to find another currency as a string from 3 upper case letters.
		matches := currencyRegex.FindAllIndex([]byte(details), -1)
		// Iterate all matches until first not skipped currency found.
		for _, match := range matches {
			// Get index of first group in regex.
			currencyIndex := match[0] + 1
			// Check that found currency is not skipped.
			if currencyIndex == skipCurrencyIndex {
				continue
			}
			// Get currency and replace targetCurrency1 or targetCurrency2.
			currency := details[currencyIndex : currencyIndex+3]
			if currency1Found {
				targetCurrency2 = currency
				currency2Index = currencyIndex
			} else {
				targetCurrency1 = currency
				currency1Index = currencyIndex
			}
			break
		}
	}
	// Again check that both currencies are found.
	if currency1Index == -1 || currency2Index == -1 {
		return nil
	}
	// Now try to find amount before each currency.
	amount1 := findAmountNearCurrency(details, currency1Index)
	amount2 := findAmountNearCurrency(details, currency2Index)
	// Check that both amounts are found.
	if amount1 == 0 || amount2 == 0 {
		return nil
	}
	// Parse exchange rate.
	// Check if there's a multiplication sign between the amounts
	minIndex := currency1Index
	if currency2Index < currency1Index {
		minIndex = currency2Index
	}
	maxIndex := currency1Index
	if currency2Index > currency1Index {
		maxIndex = currency2Index
	}
	betweenAmounts := details[minIndex:maxIndex]
	isMultiplication := strings.Contains(betweenAmounts, "*")

	var exchangeRate float64
	if isMultiplication {
		// For multiplication format: first_amount * rate = second_amount
		// So exchange_rate = second_amount / first_amount
		exchangeRate = float64(amount1) / float64(amount2)
	} else {
		// For division format: first_amount / rate = second_amount
		// So exchange_rate = first_amount / second_amount
		exchangeRate = float64(amount1) / float64(amount2)
	}

	return &ExchangeRate{
		date:         date,
		currencyFrom: targetCurrency1,
		currencyTo:   targetCurrency2,
		exchangeRate: exchangeRate,
		source:       source,
	}
}

func printCurrencyStatisticsMap(convertbleCurrencies map[string]*CurrencyStatistics) {
	if len(convertbleCurrencies) == 0 {
		fmt.Println(i18n.T("No currencies found"))
		return
	}
	fmt.Println(i18n.T("Currency\tFrom\tTo\tNumber of Exchange Rates"))
	for currency, stat := range convertbleCurrencies {
		fmt.Printf("  %s\t%s\t%s\t%d\n",
			currency,
			stat.From.Format(beancountOutputTimeFormat),
			stat.To.Format(beancountOutputTimeFormat),
			len(stat.ExchangeRates))
	}
}

func buildConversionPath(
	currencyFrom string,
	currencyTo string,
	exchangeRate float64,
	date time.Time,
	source *TransactionsSource,
) string {
	return fmt.Sprintf(
		"%s/%s=%f (at %s by '%s')",
		currencyFrom,
		currencyTo,
		exchangeRate,
		date.Format(time.DateOnly),
		source,
	)
}

// findClosestExchangeRateToCurrency finds closest to date direct exchange rate to currency.
// Advances `curState.exchangeRateIndexesPerCurrency` if exchange rate was found.
// Returns exchange rate and number of days between dates.
func findClosestExchangeRateToCurrency(
	date time.Time,
	targetCurrency string,
	curState *currencyState,
) (*ExchangeRate, int) {
	// If no exchange rates then return nil.
	if len(curState.statistics.ExchangeRates) == 0 {
		return nil, math.MaxInt
	}
	// If exchange rate index is not set then set it to 0.
	var exchangeRate *ExchangeRate = nil
	exchangeRateIndex, ok := curState.exchangeRateIndexesPerCurrency[targetCurrency]
	if !ok {
		curState.exchangeRateIndexesPerCurrency[targetCurrency] = 0
		exchangeRateIndex = 0
	}
	// If exchange rate index is out of bounds then return nil.
	if exchangeRateIndex >= len(curState.statistics.ExchangeRates) {
		return nil, math.MaxInt
	}
	exchangeRate = curState.statistics.ExchangeRates[exchangeRateIndex]
	// If another currency in exchange rate is not target currency then return nil.
	if exchangeRate.currencyTo != targetCurrency && exchangeRate.currencyFrom != targetCurrency {
		return nil, math.MaxInt
	}
	dateDiff := date.Sub(exchangeRate.date).Abs()
	// Find closest exchange rate after current one.
	for i := exchangeRateIndex + 1; i < len(curState.statistics.ExchangeRates); i++ {
		checkedEr := curState.statistics.ExchangeRates[i]
		if checkedEr.currencyTo != targetCurrency && checkedEr.currencyFrom != targetCurrency {
			// Skip not relevant exchange rates.
			continue
		}
		checkedErDateDiff := checkedEr.date.Sub(date).Abs()
		if checkedErDateDiff <= dateDiff {
			// If found something better then update exchange rate and date difference.
			exchangeRate = checkedEr
			dateDiff = checkedErDateDiff
		} else {
			// If found something worse then stop searching.
			break
		}
	}
	curState.exchangeRateIndexesPerCurrency[targetCurrency] = exchangeRateIndex
	return exchangeRate, int(dateDiff / (24 * time.Hour))
}

// convertToCurrency converts transaction amounts to convertible currencies.
// Shifts index of exchangeRateIndex in curStates because expects to be called for dates in chronological order.
// Calculates precision as:
// 0 - no conversion (amountCurrency == targetCurrency),
// 1 - with direct exchange rate to targetCurrency at the same date,
// >1 - number of days between transaction date to used exchange rate date, plus the number of days to the next exchange rate if first one was not direct.
// Returns:
// - converted amount,
// - number representing how precise conversion was,
// - path of conversion.
func convertToCurrency(
	amount MoneyWith2DecimalPlaces,
	amountCurrency string,
	targetCurrency string,
	date time.Time,
	curStates map[string]*currencyState,
) (MoneyWith2DecimalPlaces, int, []string) {
	// If the same currency then no conversion, precision is 0, path is empty.
	if amountCurrency == targetCurrency {
		return amount, 0, []string{}
	}

	// Try to find direct exchange rate.
	curState := curStates[amountCurrency]
	exchangeRateDirect, daysDiffDirect := findClosestExchangeRateToCurrency(date, targetCurrency, curState)
	if exchangeRateDirect != nil {
		precision := daysDiffDirect
		// If exchange rate is for the same date then set precision to 1, otherwise keep it as days difference.
		if precision == 0 {
			precision = 1
		}
		// Special handling for constant exchange rates.
		if exchangeRateDirect.source != nil && exchangeRateDirect.source.TypeName == ConstantExchangeRateSourceName {
			precision = ConstantExchangeRatePrecision
		}
		if exchangeRateDirect.currencyTo == targetCurrency {
			return atLeast1CentDiv(amount.int, exchangeRateDirect.exchangeRate),
				precision,
				[]string{
					buildConversionPath(
						exchangeRateDirect.currencyFrom,
						exchangeRateDirect.currencyTo,
						exchangeRateDirect.exchangeRate,
						exchangeRateDirect.date,
						exchangeRateDirect.source,
					),
				}
		}
		return atLeast1CentMul(amount.int, exchangeRateDirect.exchangeRate),
			precision,
			[]string{
				buildConversionPath(
					exchangeRateDirect.currencyTo,
					exchangeRateDirect.currencyFrom,
					1/exchangeRateDirect.exchangeRate,
					exchangeRateDirect.date,
					exchangeRateDirect.source,
				),
			}
	}

	// Otherwise try to find exchange rate by multiple conversions.
	// Use Dijkstra's algorithm to find shortest path (with minimal precision loss).
	type currencyNode struct {
		currency  string
		amount    MoneyWith2DecimalPlaces
		precision int
		path      []string // Track the conversion path with exchange rate details
	}

	// Build graph of exchange rates between currencies.
	nodes := make(map[string]*currencyNode)
	// Initialize nodes with infinity amounts except source currency.
	for currency := range curStates {
		nodes[currency] = &currencyNode{
			currency:  currency,
			amount:    MoneyWith2DecimalPlaces{int: math.MaxInt},
			precision: math.MaxInt,
			path:      []string{},
		}
	}
	// Set initial amount for source currency.
	nodes[amountCurrency] = &currencyNode{
		currency:  amountCurrency,
		amount:    amount,
		precision: 0,
		path:      []string{},
	}

	// Use priority queue to process nodes with minimal precision first.
	type queueItem struct {
		currency  string
		precision int
	}
	queue := []queueItem{{currency: amountCurrency, precision: 0}}

	// Track processed nodes to avoid cycles.
	processed := make(map[string]bool)

	// Run Dijkstra's algorithm.
	for len(queue) > 0 {
		// Get node with minimal precision
		minIdx := 0
		for i := 1; i < len(queue); i++ {
			if queue[i].precision < queue[minIdx].precision {
				minIdx = i
			}
		}
		current := queue[minIdx]
		queue = append(queue[:minIdx], queue[minIdx+1:]...)

		fromNode := nodes[current.currency]

		// Try all possible exchange rates from current currency to any other currency.
		fromCurState := curStates[current.currency]
		for _, er := range fromCurState.statistics.ExchangeRates {
			// Get the other currency from the exchange rate.
			otherCurrency := er.currencyTo
			if er.currencyTo == current.currency {
				otherCurrency = er.currencyFrom
			}

			// Skip if this rate doesn't connect to any other currency.
			if otherCurrency == current.currency {
				continue
			}

			// Calculate precision for this exchange rate step.
			daysDiff := date.Sub(er.date).Abs()
			stepPrecision := int(daysDiff / (24 * time.Hour))
			if stepPrecision == 0 {
				stepPrecision = 1 // It is not the same currency so minimal precision is 1.
			}
			// Special handling for constant exchange rates.
			if er.source != nil && er.source.TypeName == ConstantExchangeRateSourceName {
				stepPrecision = ConstantExchangeRatePrecision
			}

			// Calculate new precision as sum of all steps.
			newPrecision := fromNode.precision + stepPrecision

			// Update the other currency if we found a better path.
			otherNode := nodes[otherCurrency]
			if newPrecision < otherNode.precision {
				// Calculate converted amount based on exchange rate direction.
				var newAmount MoneyWith2DecimalPlaces
				var pathEntry string
				if current.currency == er.currencyFrom {
					newAmount = atLeast1CentDiv(fromNode.amount.int, er.exchangeRate)
					pathEntry = buildConversionPath(
						er.currencyFrom,
						er.currencyTo,
						er.exchangeRate,
						er.date,
						er.source,
					)
				} else {
					newAmount = atLeast1CentMul(fromNode.amount.int, er.exchangeRate)
					pathEntry = buildConversionPath(
						er.currencyTo,
						er.currencyFrom,
						1/er.exchangeRate,
						er.date,
						er.source,
					)
				}
				otherNode.amount = newAmount
				otherNode.precision = newPrecision
				otherNode.path = append(fromNode.path, pathEntry)
				if !processed[otherCurrency] {
					queue = append(queue, queueItem{
						currency:  otherCurrency,
						precision: newPrecision,
					})
				}
			}
		}
		processed[current.currency] = true
	}

	// Return converted amount and precision for target currency
	if targetNode, exists := nodes[targetCurrency]; exists {
		if targetNode.amount.int != math.MaxInt {
			return targetNode.amount, targetNode.precision, targetNode.path
		}
	}
	// If no conversion path found then return 0 amount with max precision.
	return MoneyWith2DecimalPlaces{int: 0}, math.MaxInt, []string{}
}

// BuildDataMart builds data required to build journal entries.
func BuildDataMart(
	transactions []Transaction,
	config *Config,
) (*DataMart, error) {
	// Sort transactions by date to simplify processing.
	slices.SortFunc(transactions, func(a, b Transaction) int {
		return a.Date.Compare(b.Date)
	})

	// Find accounts and currencies.
	accounts, currencies, err := findAccountsAndCurrencies(transactions)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, errors.New(i18n.T("no accounts found"))
	}
	if len(currencies) == 0 {
		return nil, errors.New(i18n.T("no currencies found"))
	}
	log.Println(i18n.T("In n transactions found m currencies", "n", len(transactions), "m", len(currencies)))
	printCurrencyStatisticsMap(currencies)

	// Find convertible currencies.
	convertibleCurrencies, err := buildConvertibleCurrencies(currencies, config)
	if err != nil {
		return nil, err
	}
	log.Println(i18n.T("Using n convertible currencies", "n", len(convertibleCurrencies)))
	printCurrencyStatisticsMap(convertibleCurrencies)

	return &DataMart{
		SortedTransactions:    transactions,
		Accounts:              accounts,
		AllCurrencies:         currencies,
		ConvertibleCurrencies: convertibleCurrencies,
	}, nil
}

func findAccountsAndCurrencies(transactions []Transaction) (map[string]*AccountStatistics, map[string]*CurrencyStatistics, error) {

	// Iterate all transactions to:
	// 1) validate and collect currencies (transaction may have 1 or 2 currencies), determine their timespan
	// 2) find all accounts, detemine theirs type, timespan
	// 3) make list of available exchange rates
	accounts := make(map[string]*AccountStatistics)
	currencies := make(map[string]*CurrencyStatistics)
	for _, t := range transactions {
		var exchangeRate *ExchangeRate = nil
		var isExchangeRateSet bool = false
		atLeastOneCurrency := false
		// Check account currency.
		if len(t.AccountCurrency) > 0 {
			if validCurrencyRegex.MatchString(t.AccountCurrency) {
				accountCurrency, ok := currencies[t.AccountCurrency]
				if !ok {
					accountCurrency = &CurrencyStatistics{
						Name:          t.AccountCurrency,
						From:          t.Date,
						MetInSources:  make(map[string]struct{}),
						Transactions:  []*Transaction{&t},
						ExchangeRates: []*ExchangeRate{},
					}
					currencies[t.AccountCurrency] = accountCurrency
				}
				accountCurrency.MetInSources[t.Source.FilePath] = struct{}{}
				accountCurrency.MetTimes++
				accountCurrency.To = t.Date
				// Check origin currency is present in transaction.
				if t.OriginCurrency != "" {
					accountCurrency.OverlappedWithOtherCurrencyAmount.int += t.Amount.int
					// If transaction has both currencies amounts then add exchange rate to the list.
					// Do it only once per transaction (check for OriginCurrency validity would be later).
					if t.Amount.int != 0 && t.OriginCurrencyAmount.int != 0 {
						exchangeRate = &ExchangeRate{
							date:         t.Date,
							currencyFrom: t.AccountCurrency,
							currencyTo:   t.OriginCurrency,
							exchangeRate: float64(t.Amount.int) / float64(t.OriginCurrencyAmount.int),
							source:       t.Source,
						}
						accountCurrency.ExchangeRates = append(accountCurrency.ExchangeRates, exchangeRate)
					}
				}
				accountCurrency.TotalAmount.int += t.Amount.int
				atLeastOneCurrency = true
			} else {
				return nil, nil, errors.New(
					i18n.T("invalid currency c in file f from transaction t",
						"c", t.AccountCurrency, "f", t.Source, "t", t,
					),
				)
			}
		}
		// Check origin currency.
		if t.OriginCurrency != "" {
			if validCurrencyRegex.MatchString(t.OriginCurrency) {
				// If transaction has both currencies then they should be different.
				if atLeastOneCurrency && t.OriginCurrency == t.AccountCurrency {
					return nil, nil, errors.New(
						i18n.T("transaction t has the same currency c in 'account' and 'origin'",
							"t", t, "c", t.AccountCurrency,
						),
					)
				}
				originCurrency, ok := currencies[t.OriginCurrency]
				if !ok {
					originCurrency = &CurrencyStatistics{
						Name:          t.OriginCurrency,
						From:          t.Date,
						MetInSources:  make(map[string]struct{}),
						Transactions:  []*Transaction{&t},
						ExchangeRates: []*ExchangeRate{},
					}
					currencies[t.OriginCurrency] = originCurrency
				}
				originCurrency.MetInSources[t.Source.FilePath] = struct{}{}
				originCurrency.MetTimes++
				originCurrency.To = t.Date
				// Check that transaction has account currency.
				if t.AccountCurrency != "" {
					originCurrency.OverlappedWithOtherCurrencyAmount.int += t.Amount.int
				}
				// If exchange rate is present and currency passed validation
				// then add exchange rate to the list and mark that it was set.
				if exchangeRate != nil {
					originCurrency.ExchangeRates = append(originCurrency.ExchangeRates, exchangeRate)
					isExchangeRateSet = true
				}
				originCurrency.TotalAmount.int += t.OriginCurrencyAmount.int
				atLeastOneCurrency = true
			} else {
				return nil, nil, errors.New(
					i18n.T("invalid origin currency c in file f from transaction t",
						"c", t.OriginCurrency, "f", t.Source, "t", t,
					),
				)
			}
		}
		// Check that exchange rate was set and try to parse it from details if not.
		if !isExchangeRateSet {
			// Try to parse exchange rate from details.
			exchangeRate = parseExchangeRateFromDetails(
				t.Date,
				t.Details,
				t.AccountCurrency,
				t.OriginCurrency,
				t.Source,
			)
			// If exchange rate was parsed, update both currencies. Create them if not exist.
			if exchangeRate != nil {
				// Create or update "from" currency
				fromCurrency, ok := currencies[exchangeRate.currencyFrom]
				if !ok {
					fromCurrency = &CurrencyStatistics{
						Name:          exchangeRate.currencyFrom,
						From:          t.Date,
						To:            t.Date,
						MetInSources:  make(map[string]struct{}),
						Transactions:  []*Transaction{},
						ExchangeRates: []*ExchangeRate{},
					}
					currencies[exchangeRate.currencyFrom] = fromCurrency
				}
				fromCurrency.To = t.Date
				fromCurrency.Transactions = append(fromCurrency.Transactions, &t)
				fromCurrency.ExchangeRates = append(fromCurrency.ExchangeRates, exchangeRate)

				// Create or update "to" currency
				toCurrency, ok := currencies[exchangeRate.currencyTo]
				if !ok {
					toCurrency = &CurrencyStatistics{
						Name:          exchangeRate.currencyTo,
						From:          t.Date,
						To:            t.Date,
						MetInSources:  make(map[string]struct{}),
						Transactions:  []*Transaction{},
						ExchangeRates: []*ExchangeRate{},
					}
					currencies[exchangeRate.currencyTo] = toCurrency
				}
				toCurrency.To = t.Date
				toCurrency.Transactions = append(toCurrency.Transactions, &t)
				toCurrency.ExchangeRates = append(toCurrency.ExchangeRates, exchangeRate)
			}
		}
		// Check that transaction has at least one currency.
		if !atLeastOneCurrency {
			return nil, nil, errors.New(
				i18n.T("no currency found in transaction t from file f",
					"t", t, "f", t.Source,
				),
			)
		}
		// Handle destination account.
		if len(t.ToAccount) > 0 {
			if account, ok := accounts[t.ToAccount]; !ok {
				accounts[t.ToAccount] = &AccountStatistics{
					Number:                   t.ToAccount,
					IsTransactionAccount:     !t.IsExpense,
					Source:                   t.Source,
					From:                     t.Date,
					To:                       t.Date,
					OccurencesInTransactions: 1,
					SourceOccurrences:        make(map[string]int),
				}
				// Initialize source occurrences for this account
				accounts[t.ToAccount].SourceOccurrences[fmt.Sprintf("[%s] %s", t.Source.Tag, t.Source.FilePath)] = 1
			} else {
				// Expect transactions are sorted by date.
				account.To = t.Date
				account.OccurencesInTransactions++
				// Update source occurrences
				account.SourceOccurrences[fmt.Sprintf("[%s] %s", t.Source.Tag, t.Source.FilePath)]++
				if !t.IsExpense {
					if t.Source != nil {
						account.Source = t.Source
					}
					account.IsTransactionAccount = true
				}
			}
		}
		// Handle source account.
		if len(t.FromAccount) > 0 {
			if account, ok := accounts[t.FromAccount]; !ok {
				accounts[t.FromAccount] = &AccountStatistics{
					Number:                   t.FromAccount,
					IsTransactionAccount:     t.IsExpense,
					Source:                   t.Source,
					From:                     t.Date,
					To:                       t.Date,
					OccurencesInTransactions: 1,
					SourceOccurrences:        make(map[string]int),
				}
				// Initialize source occurrences for this account
				accounts[t.FromAccount].SourceOccurrences[fmt.Sprintf("[%s] %s", t.Source.Tag, t.Source.FilePath)] = 1
			} else {
				// Expect transactions are sorted by date.
				account.To = t.Date
				account.OccurencesInTransactions++
				// Update source occurrences
				account.SourceOccurrences[fmt.Sprintf("[%s] %s", t.Source.Tag, t.Source.FilePath)]++
				if t.IsExpense {
					if t.Source != nil {
						account.Source = t.Source
					}
					account.IsTransactionAccount = true
				}
			}
		}
	}
	return accounts, currencies, nil
}

func buildConvertibleCurrencies(currencies map[string]*CurrencyStatistics, config *Config) (map[string]*CurrencyStatistics, error) {
	// Find total timespan of all currencies.
	minDate := time.Time{}
	maxDate := time.Time{}
	for _, currency := range currencies {
		// Set initial values (zero values won't work).
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
	log.Println(
		i18n.T("All transactions timespan: start..end (~m months and d days)",
			"start", minDate,
			"end", maxDate,
			"m", int(totalTimespan.Hours()/24/30),
			"d", int(totalTimespan.Hours()/24)%30,
		),
	)

	// Determine which currencies it makes sense to convert amounts into. Rules:
	// 1. Currency should span at least MinCurrencyTimespanPercent of total timespan.
	// 2. Currency should have no gaps longer than MaxCurrencyTimespanGapsDays.
	// a) Find rules.
	minTimespanPercent := config.MinCurrencyTimespanPercent
	maxGapDays := config.MaxCurrencyTimespanGapDays
	// Calculate minTimespan and maxGap in time.Duration.
	minTimespan := time.Duration(minTimespanPercent) * totalTimespan / 100
	maxGap := time.Duration(maxGapDays) * 24 * time.Hour
	// b) Iterate all currencies to check simple rules and build preliminary list of convertible currencies.
	convertibleCurrencies := map[string]*CurrencyStatistics{}
	for _, stat := range currencies {
		// Check timespan is not zero and less than minTimespan.
		currencyTimespan := stat.To.Sub(stat.From)
		if currencyTimespan > 0 && currencyTimespan < minTimespan {
			log.Println(i18n.T("Currency c has timespan t which is less than minTimespan m",
				"c", stat.Name, "t", currencyTimespan, "m", minTimespan,
			))
			continue
		}
		// Check that there are no gaps longer than maxGap for exchange rates.
		gap := maxGap
		lastTransactionDate := minDate                    // Start from first transaction date.
		for _, exchangeRate := range stat.ExchangeRates { // Could be empty.
			gap = exchangeRate.date.Sub(lastTransactionDate)
			if gap > maxGap {
				// Stop checking if gap is longer than maxGap.
				break
			}
			lastTransactionDate = exchangeRate.date
		}
		if gap > maxGap {
			log.Println(i18n.T("Currency c has gap g in 'to any currency' exchange rates which is longer than maxGap m, therefore currency is not convertible",
				"c", stat.Name, "g", gap, "m", maxGap,
			))
			continue
		}
		convertibleCurrencies[stat.Name] = stat
	}
	// c) Iterate all currencies exchange rates to check that they points to convertible currencies.
	isRecheck := true
	for isRecheck {
		isRecheck = false
		// Repeat until there are no gaps longer than maxGap for exchange rates.
		for _, stat := range convertibleCurrencies {
			gap := maxGap
			lastTransactionDate := minDate // Start from first transaction date.
			for _, exchangeRate := range stat.ExchangeRates {
				oppositeCurrency := exchangeRate.currencyFrom
				if oppositeCurrency == stat.Name {
					oppositeCurrency = exchangeRate.currencyTo
				}
				// Check that opposite currency is convertible.
				if _, ok := convertibleCurrencies[oppositeCurrency]; !ok {
					// Skip not relevant exchange rates.
					continue
				}
				// Check that there are no gaps longer than maxGap for exchange rates.
				gap = exchangeRate.date.Sub(lastTransactionDate)
				if gap > maxGap {
					// Stop checking if gap is longer than maxGap.
					break
				}
				lastTransactionDate = exchangeRate.date
			}
			// If there is a gap longer than maxGap then currency is not convertible.
			if gap > maxGap {
				log.Println(i18n.T("Currency c has gap g in 'to other convertible currencies' exchange rates which is longer than maxGap m, therefore marking currency as not convertible",
					"c", stat.Name, "g", gap, "m", maxGap,
				))
				delete(convertibleCurrencies, stat.Name)
				// Need to recheck everything one more time.
				isRecheck = true
				break
			}
		}
	}
	log.Println(
		i18n.T("With MinCurrencyTimespanPercent=m1, MaxCurrencyTimespanGapsDays=m2 getting following convertible currencies (i.e. can convert all transactions amounts with configured precision)",
			"m1", minTimespanPercent, "m2", maxGapDays,
		),
	)
	printCurrencyStatisticsMap(convertibleCurrencies)

	// If ConvertToCurrencies are provided then unconditionally add them to the list of convertible currencies.
	if len(config.ConvertToCurrencies) > 0 {
		for _, currencyName := range config.ConvertToCurrencies {
			if currencyStats, ok := currencies[currencyName]; ok {
				convertibleCurrencies[currencyName] = currencyStats
			} else {
				// If currency is not found in transactions then create artificial one.
				convertibleCurrencies[currencyName] = &CurrencyStatistics{
					Name:          currencyName,
					From:          maxDate, // Use today as the date for artificial currency.
					To:            maxDate,
					MetInSources:  make(map[string]struct{}),
					Transactions:  []*Transaction{},
					ExchangeRates: []*ExchangeRate{},
				}
				convertibleCurrencies[currencyName].MetInSources["artificial by ConvertToCurrencies"] = struct{}{}
			}
		}

		// Add constant exchange rates to transaction's ones if provided and needed.
		if len(config.ExchangeRates) > 0 {

			// Create a dummy source for constant exchange rates.
			constantSource := &TransactionsSource{
				TypeName: ConstantExchangeRateSourceName,
				Tag:      ConstantExchangeRateSourceName,
				FilePath: ConstantExchangeRateSourceFilePath,
			}

			// Build map of maps of provided exchange rates.
			cfgExchangeRates := make(map[string]map[string]*ExchangeRate)
			for currencyName, currencyMap := range config.ExchangeRates {
				for targetCurrencyName, exchangeRate := range currencyMap {
					// Add direct exchange rate.
					var directExchangeRates map[string]*ExchangeRate
					var ok bool
					if directExchangeRates, ok = cfgExchangeRates[currencyName]; !ok {
						directExchangeRates = make(map[string]*ExchangeRate)
						cfgExchangeRates[currencyName] = directExchangeRates
					}
					directExchangeRates[targetCurrencyName] = &ExchangeRate{
						date:         maxDate, // Use last transaction date.
						currencyFrom: currencyName,
						currencyTo:   targetCurrencyName,
						exchangeRate: 1 / exchangeRate,
						source:       constantSource,
					}
					// Add reverse exchange rate.
					var reverseExchangeRates map[string]*ExchangeRate
					if reverseExchangeRates, ok = cfgExchangeRates[targetCurrencyName]; !ok {
						reverseExchangeRates = make(map[string]*ExchangeRate)
						cfgExchangeRates[targetCurrencyName] = reverseExchangeRates
					}
					reverseExchangeRates[currencyName] = &ExchangeRate{
						date:         maxDate, // Use last transaction date.
						currencyFrom: targetCurrencyName,
						currencyTo:   currencyName,
						exchangeRate: exchangeRate,
						source:       constantSource,
					}
				}
			}

			// Iterate all convertible currencies to check if any exchange rates are available for them.
			// Note that multi-hop conversions are not checked here.
			for _, currencyStat := range convertibleCurrencies {
				// Check if any exchange rate for this currency is provided in transactions.
				if len(currencyStat.ExchangeRates) > 0 {
					continue
				}
				log.Println(i18n.T("No exchange rates for currency c found in provided transactions, trying to find exchange rates from ExchangeRates structure instead", "c", currencyStat.Name))
				// Try to find exchange rates for this currency in cfgExchangeRates.
				if currencyMap, ok := cfgExchangeRates[currencyStat.Name]; ok {
					for _, exchangeRate := range currencyMap {
						// Add exchange rates to all currencies.
						currencyStat.ExchangeRates = append(currencyStat.ExchangeRates, exchangeRate)
					}
					continue
				}
				return nil, errors.New(
					i18n.T("Any exchange rates for currency c from ConvertToCurrencies not found both in provided transactions and in ExchangeRates structure",
						"c", currencyStat.Name,
					),
				)
			}
		}
	}

	// Check that we end up with at least one convertible currency.
	if len(convertibleCurrencies) == 0 {
		return nil, errors.New(
			i18n.T("Convertible currencies not found due to too few exchange rates. Consider change config file with (in priority order):\n"+
				"- decreasing MinCurrencyTimespanPercent and/or increasing MaxCurrencyTimespanGapsDays,\n"+
				"- adding ConvertToCurrencies list of currencies to ignore precision checks,\n",
				"- adding ExchangeRates structure to provide constant exchange rate.",
			),
		)
	}
	return convertibleCurrencies, nil
}

// buildJournalEntries builds journal entries from transactions.
// Returns journal entries, uncategorized transactions, error.
func buildJournalEntries(
	dataMart *DataMart,
	categorization *Categorization,
) ([]JournalEntry, []Transaction, error) {

	// Make map of currencyState to speed up conversions.
	curStates := make(map[string]*currencyState, len(dataMart.AllCurrencies))
	for currency, statistics := range dataMart.AllCurrencies {
		curStates[currency] = &currencyState{
			currency:                       currency,
			statistics:                     statistics,
			exchangeRateIndexesPerCurrency: make(map[string]int),
		}
	}
	log.Println(
		i18n.T("All d exchange rates will be used for conversions as a 'best effort'",
			"d", len(curStates),
		),
	)
	log.Println(
		i18n.T(
			"Building journal entries with using exchange rates from alln currencies and converting to these n currencies",
			"alln", len(curStates),
			"n", len(dataMart.ConvertibleCurrencies),
		),
	)
	printCurrencyStatisticsMap(dataMart.ConvertibleCurrencies)

	journalEntries := []JournalEntry{}
	uncategorizedTransactions := []Transaction{}
	for _, t := range dataMart.SortedTransactions {
		// Try to find category using Categorization instance.
		category, isUncategorized, err := categorization.CategorizeTransaction(&t)
		if err != nil {
			return nil, nil, err
		} else if isUncategorized {
			uncategorizedTransactions = append(uncategorizedTransactions, t)
		}
		// Convert amounts to convertible currencies.
		amounts := make(map[string]AmountInCurrency, len(dataMart.ConvertibleCurrencies))
		for _, curStatistic := range dataMart.ConvertibleCurrencies {
			var amountAccCur, amountOrgCur MoneyWith2DecimalPlaces
			var precisionAccCur, precisionOrgCur int = math.MaxInt, math.MaxInt
			var conversionPathAccCur, conversionPathOrgCur []string

			// First convert account currency amount if it exists and amount is non-zero.
			if t.AccountCurrency != "" && t.Amount.int != 0 {
				amountAccCur, precisionAccCur, conversionPathAccCur = convertToCurrency(t.Amount, t.AccountCurrency, curStatistic.Name, t.Date, curStates)
			}

			// Then convert origin currency amount if it exists and amount is non-zero.
			if t.OriginCurrency != "" && t.OriginCurrencyAmount.int != 0 {
				amountOrgCur, precisionOrgCur, conversionPathOrgCur = convertToCurrency(t.OriginCurrencyAmount, t.OriginCurrency, curStatistic.Name, t.Date, curStates)
			}

			// Check that any amount is non-zero (i.e. conversion was successful).
			if amountAccCur.int == 0 && amountOrgCur.int == 0 {
				accountCurrencyRatesDesc := ""
				if _, ok := curStates[t.AccountCurrency]; ok {
					accountCurrencyRatesDesc = fmt.Sprintf("%+v", curStates[t.AccountCurrency].statistics.ExchangeRates)
				}
				originCurrencyRatesDesc := ""
				if _, ok := curStates[t.OriginCurrency]; ok {
					originCurrencyRatesDesc = fmt.Sprintf("%+v", curStates[t.OriginCurrency].statistics.ExchangeRates)
				}
				return nil, nil, errors.New(
					i18n.T(
						"transaction t can't be converted to c currency because not enough exchange rates were found to connect transaction currency with c currency. Having only following exchange rates for account currency: rates1 and for origin currency: rates2",
						"t", t,
						"c", curStatistic.Name,
						"rates1", accountCurrencyRatesDesc,
						"rates2", originCurrencyRatesDesc,
					),
				)
			}

			// Use the conversion with better precision.
			if precisionAccCur <= precisionOrgCur {
				amounts[curStatistic.Name] = AmountInCurrency{
					Currency:            curStatistic.Name,
					Amount:              amountAccCur,
					ConversionPrecision: precisionAccCur,
					ConversionPath:      conversionPathAccCur,
				}
			} else {
				amounts[curStatistic.Name] = AmountInCurrency{
					Currency:            curStatistic.Name,
					Amount:              amountOrgCur,
					ConversionPrecision: precisionOrgCur,
					ConversionPath:      conversionPathOrgCur,
				}
			}
		}
		// Try to set "account currency amount" even if it wasn't provided in transaction.
		amount := t.Amount
		if amount.int == 0 {
			if convertedAmount, ok := amounts[t.AccountCurrency]; ok {
				amount = convertedAmount.Amount
			} else {
				return nil, nil, errors.New(
					i18n.T(
						"transaction t amount in account currency c can't be set because both origin file doesn't provide it and currency haven't choosen for conversion into",
						"t", t,
						"c", t.AccountCurrency,
					),
				)
			}
		}
		entry := JournalEntry{
			Date:                  t.Date,
			IsExpense:             t.IsExpense,
			Source:                t.Source,
			Details:               t.Details,
			Category:              category.Name,
			AccountCurrency:       t.AccountCurrency,
			AccountCurrencyAmount: amount,
			OriginCurrency:        t.OriginCurrency,
			OriginCurrencyAmount:  t.OriginCurrencyAmount,
			FromAccount:           t.FromAccount,
			ToAccount:             t.ToAccount,
			Amounts:               amounts,
			RuleType:              category.RuleType,
			RuleValue:             category.RuleValue,
		}
		journalEntries = append(journalEntries, entry)
	}

	log.Println(
		i18n.T(
			"Total assembled n journal entries with amounts in m currencies, n2 uncategorized transactions",
			"n", len(journalEntries), "m", len(dataMart.ConvertibleCurrencies), "n2", len(uncategorizedTransactions),
		),
	)
	return journalEntries, uncategorizedTransactions, nil
}
