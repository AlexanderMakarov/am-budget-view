package bankdownload

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AlexanderMakarov/am-budget-view/internal/config"
)

const (
	myAmeriaAPIBase    = "https://ob.myameria.am"
	myAmeriaTimeout    = 30 * time.Second
	myAmeriaOutputDate = "2006-01-02"
)

var genericCSVHeaders = []string{
	"Date",
	"FromAccount",
	"ToAccount",
	"IsExpense",
	"Amount",
	"Details",
	"AccountCurrency",
	"OriginCurrency",
	"OriginCurrencyAmount",
}

type myAmeriaAmount struct {
	Currency string  `json:"currency"`
	Amount   float64 `json:"amount"`
}

type myAmeriaHistoryEntry struct {
	ID                  string         `json:"id"`
	OperationDate       string         `json:"operationDate"`
	DebitAccountNumber  string         `json:"debitAccountNumber"`
	CreditAccountNumber string         `json:"creditAccountNumber"`
	Details             string         `json:"details"`
	Amount              myAmeriaAmount `json:"amount"`
	AccountingType      string         `json:"accountingType"`
	TransactionType     string         `json:"transactionType"`
}

type myAmeriaHistoryResponse struct {
	Data struct {
		Entries []myAmeriaHistoryEntry `json:"entries"`
	} `json:"data"`
}

type myAmeriaClient struct {
	httpClient *http.Client
	apiBase    string
}

func newMyAmeriaClient(httpClient *http.Client) *myAmeriaClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: myAmeriaTimeout}
	}
	return &myAmeriaClient{
		httpClient: httpClient,
		apiBase:    myAmeriaAPIBase,
	}
}

func myAmeriaHeaders(clientID, authToken string, now time.Time) http.Header {
	_, offset := now.Zone()
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("Authorization", NormalizeMyAmeriaAuthToken(authToken))
	h.Set("Client-Time", now.Format("15:04:05"))
	h.Set("Client-Id", clientID)
	h.Set("Locale", "en")
	h.Set("Timezone-Offset", strconv.Itoa(offset/60))
	return h
}

// NormalizeMyAmeriaAuthToken formats a value for the Authorization header.
// Accepts a raw JWT or the full header copied from DevTools (Bearer …).
func NormalizeMyAmeriaAuthToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return "Bearer " + strings.TrimSpace(token[7:])
	}
	return "Bearer " + token
}

func myAmeriaHistoryOutputPath(wd, sinceDate string) (string, error) {
	t, err := time.Parse(sinceDateLayout, sinceDate)
	if err != nil {
		return "", fmt.Errorf("invalid sinceDate %q: must be DD-MM-YYYY: %w", sinceDate, err)
	}
	return filepath.Join(
		wd,
		fmt.Sprintf("generic MyAmeria History since %s.csv", t.Format(myAmeriaOutputDate)),
	), nil
}

func slashEncodeDate(ddmmyyyy string) string {
	return strings.ReplaceAll(ddmmyyyy, "-", "%2F")
}

func (c *myAmeriaClient) fetchHistory(clientID, authToken, fromDate, toDate string) ([]myAmeriaHistoryEntry, error) {
	now := time.Now()
	url := fmt.Sprintf(
		"%s/api/events/past?locale=en&toAmount=10000000000&fromDate=%s&toDate=%s&sort=date&size=10000&page=1",
		c.apiBase,
		slashEncodeDate(fromDate),
		slashEncodeDate(toDate),
	)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header = myAmeriaHeaders(clientID, authToken, now)

	resp, err := c.httpClient.Do(req)
	var body []byte
	if resp != nil {
		defer resp.Body.Close()
		body, err = io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	}
	logHTTPExchange("MyAmeria", req.Method, req.URL.String(), req.Header, resp, body, err)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &myAmeriaHTTPError{
			statusCode: resp.StatusCode,
			body:       strings.TrimSpace(string(body)),
		}
	}

	var payload myAmeriaHistoryResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("MyAmeria history response is not valid JSON: %w", err)
	}
	return payload.Data.Entries, nil
}

type accountCurrencyKey struct {
	account  string
	currency string
}

func convertMyAmeriaHistoryEntries(entries []myAmeriaHistoryEntry) (map[accountCurrencyKey][]myAmeriaHistoryEntry, error) {
	myAccounts := make(map[accountCurrencyKey]struct{})

	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		currency := entry.Amount.Currency
		debitAccount := entry.DebitAccountNumber
		creditAccount := entry.CreditAccountNumber

		switch entry.TransactionType {
		case "transfer:between-own-accounts", "transfer:local":
			if entry.AccountingType == "DEBIT" {
				myAccounts[accountCurrencyKey{debitAccount, currency}] = struct{}{}
			} else {
				myAccounts[accountCurrencyKey{creditAccount, currency}] = struct{}{}
			}
		case "exchange":
			if entry.AccountingType == "DEBIT" {
				myAccounts[accountCurrencyKey{debitAccount, currency}] = struct{}{}
			} else {
				myAccounts[accountCurrencyKey{creditAccount, currency}] = struct{}{}
			}
		case "card", "transfer:to-card", "transfer:international",
			"charge:commission:transfer", "charge:commission",
			"charge:international", "cash-out":
			if entry.AccountingType == "DEBIT" {
				myAccounts[accountCurrencyKey{debitAccount, currency}] = struct{}{}
			} else {
				myAccounts[accountCurrencyKey{creditAccount, currency}] = struct{}{}
			}
		case "deposit", "deposit:cash", "deposit:replenishment":
			myAccounts[accountCurrencyKey{creditAccount, currency}] = struct{}{}
		default:
			return nil, fmt.Errorf("unknown transaction type: %s", entry.TransactionType)
		}
	}

	myAccountNumbers := make(map[string]struct{})
	for key := range myAccounts {
		myAccountNumbers[key.account] = struct{}{}
	}

	accountTransactions := make(map[string][]myAmeriaHistoryEntry)
	for _, entry := range entries {
		debitAccount := entry.DebitAccountNumber
		creditAccount := entry.CreditAccountNumber
		transactionAssigned := false

		if entry.AccountingType == "DEBIT" {
			if _, ok := myAccountNumbers[debitAccount]; ok {
				accountTransactions[debitAccount] = append(accountTransactions[debitAccount], entry)
				transactionAssigned = true
			}
		}
		if entry.AccountingType == "CREDIT" {
			if _, ok := myAccountNumbers[creditAccount]; ok {
				accountTransactions[creditAccount] = append(accountTransactions[creditAccount], entry)
				transactionAssigned = true
			}
		}
		if !transactionAssigned {
			return nil, fmt.Errorf(
				"transaction %s doesn't belong to any of my accounts",
				entry.ID,
			)
		}
	}

	result := make(map[accountCurrencyKey][]myAmeriaHistoryEntry)
	for account, transactions := range accountTransactions {
		var accountCurrencies []string
		for key := range myAccounts {
			if key.account == account {
				accountCurrencies = append(accountCurrencies, key.currency)
			}
		}
		if len(accountCurrencies) != 1 {
			return nil, fmt.Errorf("could not find currency for account %s", account)
		}
		result[accountCurrencyKey{account, accountCurrencies[0]}] = transactions
	}

	totalTransactions := 0
	for _, txs := range result {
		totalTransactions += len(txs)
	}
	if totalTransactions != len(entries) {
		return nil, fmt.Errorf(
			"transactions are duplicated between my accounts: %d != %d",
			totalTransactions,
			len(entries),
		)
	}
	return result, nil
}

func writeMyAmeriaGenericCSV(path string, accounts map[accountCurrencyKey][]myAmeriaHistoryEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(genericCSVHeaders); err != nil {
		return err
	}

	for key, transactions := range accounts {
		nativeCurrency := key.currency
		for i, entry := range transactions {
			operationDate, err := parseMyAmeriaOperationDate(entry.OperationDate)
			if err != nil {
				return fmt.Errorf("entry %s: %w", entry.ID, err)
			}

			isExpense := entry.AccountingType == "DEBIT"
			debitAccount := entry.DebitAccountNumber
			creditAccount := entry.CreditAccountNumber

			transactionCurrency := entry.Amount.Currency
			transactionAmount := entry.Amount.Amount

			var accountAmount float64
			var accountCurrency, originCurrency, originAmount string

			if transactionCurrency == nativeCurrency {
				accountAmount = transactionAmount
				accountCurrency = nativeCurrency
			} else {
				accountAmount = 0
				accountCurrency = nativeCurrency
				originCurrency = transactionCurrency
				originAmount = fmt.Sprintf("%.2f", transactionAmount)
			}

			if accountAmount <= 0 {
				return fmt.Errorf(
					"%d line: wrong amount '%v' for entry %s",
					i,
					accountAmount,
					entry.ID,
				)
			}

			if err := w.Write([]string{
				operationDate,
				debitAccount,
				creditAccount,
				strconv.FormatBool(isExpense),
				fmt.Sprintf("%.2f", accountAmount),
				entry.Details,
				accountCurrency,
				originCurrency,
				originAmount,
			}); err != nil {
				return err
			}
		}
	}

	w.Flush()
	return w.Error()
}

func parseMyAmeriaOperationDate(raw string) (string, error) {
	normalized := strings.Replace(raw, "Z", "+00:00", 1)
	t, err := time.Parse(time.RFC3339Nano, normalized)
	if err != nil {
		t, err = time.Parse(time.RFC3339, normalized)
		if err != nil {
			return "", fmt.Errorf("invalid operationDate %q: %w", raw, err)
		}
	}
	return t.Format(myAmeriaOutputDate), nil
}

// DownloadMyAmeria downloads all-account history from ob.myameria.am and writes generic CSV.
func DownloadMyAmeria(cfg config.MyAmeriaDownloadConfig, wd string) (string, error) {
	return downloadMyAmeria(cfg, wd, newMyAmeriaClient(nil))
}

func downloadMyAmeria(cfg config.MyAmeriaDownloadConfig, wd string, client *myAmeriaClient) (string, error) {
	if cfg.ClientId == "" {
		return "", fmt.Errorf("my ameria clientId is required — open Download settings and set Client-Id from DevTools")
	}
	authToken := NormalizeMyAmeriaAuthToken(cfg.AuthToken)
	if authToken == "" {
		return "", fmt.Errorf("my ameria authToken is required — paste the Authorization header from DevTools (Bearer …)")
	}
	if cfg.SinceDate == "" {
		return "", fmt.Errorf("my ameria sinceDate is required — set start date in Download settings (DD-MM-YYYY)")
	}

	outPath, err := myAmeriaHistoryOutputPath(wd, cfg.SinceDate)
	if err != nil {
		return "", err
	}

	toDate := time.Now().Format(sinceDateLayout)
	log.Printf("MyAmeria: downloading history to %q", outPath)
	entries, err := client.fetchHistory(cfg.ClientId, authToken, cfg.SinceDate, toDate)
	if err != nil {
		return "", err
	}
	log.Printf("MyAmeria: received %d history entries", len(entries))

	accounts, err := convertMyAmeriaHistoryEntries(entries)
	if err != nil {
		return "", err
	}

	if err := writeMyAmeriaGenericCSV(outPath, accounts); err != nil {
		return "", err
	}
	log.Printf("MyAmeria: wrote %q (%d account groups)", outPath, len(accounts))
	return outPath, nil
}
