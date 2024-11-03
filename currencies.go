package main

import (
	"fmt"
	"log"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

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
	exchangeRateIndexesPerCurrency map[string]int
}

// findAmountNearCurrency searches a number before specified index in details.
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
// Transaction details may contain it in format "330000 AMD / 4.4 = 75000 RUB".
func parseExchangeRateFromDetails(date time.Time, details string, targetCurrency1, targetCurrency2 string) *ExchangeRate {
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
	exchangeRate := float64(amount2) / float64(amount1)
	return &ExchangeRate{
		date:         date,
		currencyFrom: targetCurrency1,
		currencyTo:   targetCurrency2,
		exchangeRate: exchangeRate,
	}
}

// findClosestExchangeRate finds closest exchange rate to the given date.
// Not uses `curState.exchangeRateIndexesPerCurrency`.
// returns precision as number of days between dates.
func findClosestExchangeRate(
	date time.Time,
	curState *currencyState,
) (int) {
	if len(curState.statistics.ExchangeRates) == 0 {
		return math.MaxInt
	}
	var dateDiff time.Duration = math.MaxInt
	// Find closest exchange rate after current one.
	for i := 0; i < len(curState.statistics.ExchangeRates); i++ {
		checkedEr := curState.statistics.ExchangeRates[i]
		if date.Sub(checkedEr.date).Abs() < dateDiff {
			dateDiff = date.Sub(checkedEr.date).Abs()
		}
	}
	return int(dateDiff / (24 * time.Hour))
}

// findClosestExchangeRateToCurrency finds closest to date direct exchange rate to currency.
// Advances `curState.exchangeRateIndexesPerCurrency` if exchange rate was found.
// Returns exchange rate and number of days between dates.
func findClosestExchangeRateToCurrency(
	date time.Time,
	targetCurrency string,
	curState *currencyState,
) (*ExchangeRate, int) {
	if len(curState.statistics.ExchangeRates) == 0 {
		return nil, math.MaxInt
	}
	var exchangeRate *ExchangeRate = nil
	exchangeRateIndex, ok := curState.exchangeRateIndexesPerCurrency[targetCurrency]
	if !ok {
		return nil, math.MaxInt
	}
	dateDiff := date.Sub(exchangeRate.date).Abs()
	// Find closest exchange rate after current one.
	for i := exchangeRateIndex; i < len(curState.statistics.ExchangeRates); i++ {
		checkedEr := curState.statistics.ExchangeRates[i]
		if checkedEr.currencyTo != targetCurrency && checkedEr.currencyFrom != targetCurrency {
			// Skip not relevant exchange rates.
			continue
		}
		checkedErDateDiff := checkedEr.date.Sub(date).Abs()
		if checkedErDateDiff < dateDiff {
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
	exchangeRateDirect, daysDiffDirect := findClosestExchangeRateToCurrency(date, targetCurrency, &curState)
	if exchangeRateDirect != nil {
		precision := daysDiffDirect
		// If exchange rate is for the same date then set precision to 1, otherwise keep it as days difference.
		if precision == 0 {
			precision = 1
		}
		if exchangeRateDirect.currencyTo == targetCurrency {
			return MoneyWith2DecimalPlaces{int: int(float64(amount.int) * exchangeRateDirect.exchangeRate)}, precision
		}
		return MoneyWith2DecimalPlaces{int: int(float64(amount.int) / exchangeRateDirect.exchangeRate)}, precision
	}

	// Otherwise try to find exchange rate by multiple conversions.
	// Use Dijkstra's algorithm to find shortest path (with minimal precision loss).
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

	// Use priority queue to process nodes with minimal precision first
	type queueItem struct {
		currency  string
		precision int
	}
	queue := []queueItem{{currency: amountCurrency, precision: 0}}

	// Track processed nodes to avoid cycles
	processed := make(map[string]bool)

	// Run Dijkstra's algorithm
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

		// Try all possible exchange rates from current currency to any other currency
		for toCurrency := range curStates {
			if toCurrency == current.currency {
				continue
			}

			// Find best exchange rate between currencies
			bestPrecision := math.MaxInt
			var bestAmount MoneyWith2DecimalPlaces

			// Check direct rates from current currency
			fromCurState := curStates[current.currency]
			for _, er := range fromCurState.statistics.ExchangeRates {
				if er.currencyTo != toCurrency && er.currencyFrom != toCurrency {
					continue
				}

				// Find closest exchange rate to date
				daysDiff := findClosestExchangeRate(date, &fromCurState)

				// Calculate new precision
				newPrecision := daysDiff
				if newPrecision == 0 {
					newPrecision = 1
				}
				newPrecision += fromNode.precision

				// Calculate converted amount based on exchange rate direction
				// ExchangeRate = CurrencyFrom / CurrencyTo
				var newAmount MoneyWith2DecimalPlaces
				if er.currencyFrom == current.currency && er.currencyTo == toCurrency {
					// Direct conversion: current -> target
					newAmount = MoneyWith2DecimalPlaces{
						int: int(float64(fromNode.amount.int) * er.exchangeRate),
					}
				} else if er.currencyTo == current.currency && er.currencyFrom == toCurrency {
					// Reverse conversion: target -> current
					newAmount = MoneyWith2DecimalPlaces{
						int: int(float64(fromNode.amount.int) * er.exchangeRate),
					}
				} else if er.currencyFrom == current.currency {
					// Indirect conversion through another currency
					newAmount = MoneyWith2DecimalPlaces{
						int: int(float64(fromNode.amount.int) * er.exchangeRate),
					}
				} else {
					// Reverse indirect conversion
					newAmount = MoneyWith2DecimalPlaces{
						int: int(float64(fromNode.amount.int) / er.exchangeRate),
					}
				}

				if newPrecision < bestPrecision {
					bestPrecision = newPrecision
					bestAmount = newAmount
				}
			}

			// Update target node if found better precision
			toNode := nodes[toCurrency]
			if bestPrecision < toNode.precision {
				toNode.amount = bestAmount
				toNode.precision = bestPrecision
				// Add to queue if not processed
				if !processed[toCurrency] {
					queue = append(queue, queueItem{
						currency:  toCurrency,
						precision: bestPrecision,
					})
				}
			}
		}

		processed[current.currency] = true
	}

	// Return converted amount and precision for target currency
	if targetNode, exists := nodes[targetCurrency]; exists {
		if targetNode.amount.int != math.MaxInt {
			return targetNode.amount, targetNode.precision
		}
	}
	// If no conversion path found then return original amount with max precision
	return amount, math.MaxInt
}

// buildJournalEntries builds journal entries from transactions.
// It handles all currencies conversion basing on exchange rates found in transactions only.
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
				accountCurrency.MetInSources[t.SourceType] = struct{}{}
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
						}
						accountCurrency.ExchangeRates = append(accountCurrency.ExchangeRates, exchangeRate)
					}
				}
				accountCurrency.TotalAmount.int += t.Amount.int
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
						"transaction '%+v' has the same currency '%s' as 'account' and 'origin'",
						t, t.AccountCurrency,
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
				originCurrency.MetInSources[t.SourceType] = struct{}{}
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
				return nil, nil, nil, fmt.Errorf(
					"invalid origin currency '%s' in file '%s' from transaction: %+v",
					t.OriginCurrency, t.Source, t,
				)
			}
		}
		// Check that exchange rate was set and try to parse it from details if not.
		if !isExchangeRateSet {
			// Try to parse exchange rate from details.
			exchangeRate = parseExchangeRateFromDetails(t.Date, t.Details, t.AccountCurrency, t.OriginCurrency)
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
	log.Printf("In %d transactions found %d currencies:\n", len(transactions), len(currencies))
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
	log.Printf(
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
			log.Printf("Currency '%s' has timespan %s which is less than minTimespan %s\n", stat.Name, stat.To.Sub(stat.From), minTimespan)
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
			log.Printf("Currency '%s' has gap in 'any' exchange rates %s which is longer than maxGap %s\n", stat.Name, hasGapAtLeastDays, maxGap)
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
				log.Printf("Currency '%s' has gap in 'to convertible currencies' exchange rates %s which is longer than maxGap %s\n", stat.Name, hasGapAtLeastDays, maxGap)
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
	log.Printf(
		"With MinCurrencyTimespanPercent=%d, MaxCurrencyTimespanGapsDays=%d filtered out following currencies to convert all transactions amounts into:\n",
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
		return nil, nil, nil, fmt.Errorf(
			"'good' convertable currencies not found, consider change config file with %s, %s, %s",
			"a) adding ConvertToCurrencies entry (i.e. try convert unconditionally to some currency)",
			"b) decreasing MinCurrencyTimespanPercent",
			"c) increasing MaxCurrencyTimespanGapsDays",
		)
	}

	// Make list of currency states to speed up conversions.
	curStates := make(map[string]currencyState, len(currencies))
	for currency := range currencies {
		statistics := currencies[currency]
		curStates[currency] = currencyState{
			currency:          currency,
			statistics:        statistics,
			exchangeRateIndexesPerCurrency: make(map[string]int),
		}
	}
	log.Printf("Building journal entries with conversions to %d currencies:\n", len(convertableCurrencies))
	printCurrencyStatisticsMap(convertableCurrencies)
	log.Printf("All %d exchange rates will be used for conversions as a 'best effort'.\n", len(curStates))

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
		// Otherwise add transaction to either "Unknown" or personal group.
		if category == nil {
			// Choose name of group to add transaction into.
			if config.GroupAllUnknownTransactions {
				unknownCategory := UnknownGroupName
				category = &unknownCategory
			} else {
				category = &t.Details
			}
		}
		// Convert amounts to convertable currencies.
		amounts := make(map[string]AmountInCurrency, len(convertableCurrencies))
		for _, curStatistic := range convertableCurrencies {
			var amount1, amount2 MoneyWith2DecimalPlaces
			var precision1, precision2 int = math.MaxInt, math.MaxInt

			// Only convert if currency exists and amount is non-zero. Use all available exchange rates.
			if t.AccountCurrency != "" && t.Amount.int != 0 {
				amount1, precision1 = convertToCurrency(t.Amount, t.AccountCurrency, curStatistic.Name, t.Date, curStates)
			}

			// Only convert if currency exists and amount is non-zero. Use all available exchange rates.
			if t.OriginCurrency != "" && t.OriginCurrencyAmount.int != 0 {
				amount2, precision2 = convertToCurrency(t.OriginCurrencyAmount, t.OriginCurrency, curStatistic.Name, t.Date, curStates)
			}

			// Check that any amount is non-zero.
			// Note that if transaction amount is 1 (i.e. 100 in 'MoneyWith2DecimalPlaces.int' property)
			// it may be converted to another currency as 0 but it is valid conversion.
			// Such transactions are used to check that card can be charged in general.
			if amount1.int == 0 && amount2.int == 0 && (t.Amount.int != 100 && t.OriginCurrencyAmount.int != 100) {
				return nil, nil, nil, fmt.Errorf(
					"transaction '%+v' can't be converted to %s currency, not enough exchange rates found to connect transaction currency with %s currency",
					t, curStatistic.Name, curStatistic.Name,
				)
			}

			// Use the conversion with better precision.
			if precision1 <= precision2 {
				amounts[curStatistic.Name] = AmountInCurrency{
					Currency:            curStatistic.Name,
					Amount:              amount1,
					ConversionPrecision: precision1,
				}
			} else {
				amounts[curStatistic.Name] = AmountInCurrency{
					Currency:            curStatistic.Name,
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

	log.Printf("Total assembled %d journal entries with amounts in %d currencies.\n", len(journalEntries), len(curStates))
	return journalEntries, accounts, currencies, nil
}
