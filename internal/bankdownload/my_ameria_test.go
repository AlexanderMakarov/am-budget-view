package bankdownload

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexanderMakarov/am-budget-view/internal/config"
	"github.com/AlexanderMakarov/am-budget-view/internal/parser"
)

func TestConvertMyAmeriaHistoryEntries(t *testing.T) {
	entries := []myAmeriaHistoryEntry{
		{
			ID:                  "income-1",
			OperationDate:       "2024-03-01T10:00:00.000Z",
			DebitAccountNumber:  "9999999999999999",
			CreditAccountNumber: "1111111111111111",
			Details:             "Salary",
			Amount:              myAmeriaAmount{Currency: "AMD", Amount: 100000},
			AccountingType:      "CREDIT",
			TransactionType:     "deposit",
		},
		{
			ID:                  "expense-1",
			OperationDate:       "2024-03-02T11:00:00.000Z",
			DebitAccountNumber:  "1111111111111111",
			CreditAccountNumber: "2222222222222222",
			Details:             "Coffee",
			Amount:              myAmeriaAmount{Currency: "AMD", Amount: 1500},
			AccountingType:      "DEBIT",
			TransactionType:     "card",
		},
	}

	accounts, err := convertMyAmeriaHistoryEntries(entries)
	if err != nil {
		t.Fatalf("convertMyAmeriaHistoryEntries: %v", err)
	}

	key := accountCurrencyKey{account: "1111111111111111", currency: "AMD"}
	txs, ok := accounts[key]
	if !ok {
		t.Fatalf("expected account %v in result, got %#v", key, accounts)
	}
	if len(txs) != 2 {
		t.Fatalf("expected 2 transactions for account, got %d", len(txs))
	}
}

func TestConvertMyAmeriaHistoryEntries_UnknownType(t *testing.T) {
	entries := []myAmeriaHistoryEntry{
		{
			ID:              "bad-1",
			TransactionType: "unknown:type",
			AccountingType:  "DEBIT",
			Amount:          myAmeriaAmount{Currency: "AMD", Amount: 1},
		},
	}
	_, err := convertMyAmeriaHistoryEntries(entries)
	if err == nil || !strings.Contains(err.Error(), "unknown transaction type") {
		t.Fatalf("expected unknown transaction type error, got %v", err)
	}
}

func TestNormalizeMyAmeriaAuthToken(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"  ", ""},
		{"eyJhbGciOi", "Bearer eyJhbGciOi"},
		{"Bearer eyJhbGciOi", "Bearer eyJhbGciOi"},
		{"bearer eyJhbGciOi", "Bearer eyJhbGciOi"},
		{"  Bearer  eyJhbGciOi  ", "Bearer eyJhbGciOi"},
	}
	for _, tc := range cases {
		got := NormalizeMyAmeriaAuthToken(tc.in)
		if got != tc.want {
			t.Errorf("NormalizeMyAmeriaAuthToken(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDownloadMyAmeria_Success(t *testing.T) {
	payload := map[string]any{
		"data": map[string]any{
			"entries": []map[string]any{
				{
					"id":                  "tx-1",
					"operationDate":       "2024-04-10T12:00:00.000Z",
					"debitAccountNumber":  "1570082942569400",
					"creditAccountNumber": "1570017510050100",
					"details":             "Test purchase",
					"amount": map[string]any{
						"currency": "AMD",
						"amount":   2500.5,
					},
					"accountingType":  "DEBIT",
					"transactionType": "card",
				},
				{
					"id":                  "tx-2",
					"operationDate":       "2024-04-11T08:00:00.000Z",
					"debitAccountNumber":  "9999999999999999",
					"creditAccountNumber": "1570082942569400",
					"details":             "Salary",
					"amount": map[string]any{
						"currency": "AMD",
						"amount":   50000,
					},
					"accountingType":  "CREDIT",
					"transactionType": "deposit",
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/events/past" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Client-Id"); got != "test-client-id" {
			t.Errorf("Client-Id = %q, want test-client-id", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", got)
		}
		if !strings.Contains(r.URL.RawQuery, "fromDate=01%2F04%2F2024") {
			t.Errorf("fromDate not encoded in query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	client := newMyAmeriaClient(server.Client())
	client.apiBase = server.URL

	wd := t.TempDir()
	cfg := config.MyAmeriaDownloadConfig{
		Enabled:   true,
		ClientId:  "test-client-id",
		AuthToken: "test-token",
		SinceDate: "01-04-2024",
	}

	path, err := downloadMyAmeria(cfg, wd, client)
	if err != nil {
		t.Fatalf("downloadMyAmeria: %v", err)
	}

	wantPath := filepath.Join(wd, "generic MyAmeria History since 2024-04-01.csv")
	if path != wantPath {
		t.Fatalf("path = %q, want %q", path, wantPath)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	records, err := csv.NewReader(strings.NewReader(string(data))).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected header + 2 rows, got %d rows", len(records))
	}
	if records[0][0] != "Date" {
		t.Fatalf("unexpected header: %v", records[0])
	}

	transactions, err := parser.GenericCsvFileParser{}.ParseRawTransactionsFromFile(path)
	if err != nil {
		t.Fatalf("generic csv parser: %v", err)
	}
	if len(transactions) != 2 {
		t.Fatalf("expected 2 parsed transactions, got %d", len(transactions))
	}
}

func TestDownloadMyAmeria_WorksWithoutEnabledFlag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"entries": []any{}}})
	}))
	defer server.Close()

	client := newMyAmeriaClient(server.Client())
	client.apiBase = server.URL

	cfg := config.MyAmeriaDownloadConfig{
		Enabled:   false,
		ClientId:  "client",
		AuthToken: "Bearer tok",
		SinceDate: "01-04-2024",
	}
	_, err := downloadMyAmeria(cfg, t.TempDir(), client)
	if err != nil {
		t.Fatalf("downloadMyAmeria with Enabled=false: %v", err)
	}
}

func TestDownloadMyAmeria_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	client := newMyAmeriaClient(server.Client())
	client.apiBase = server.URL

	cfg := config.MyAmeriaDownloadConfig{
		Enabled:   true,
		ClientId:  "test-client-id",
		AuthToken: "Bearer expired",
		SinceDate: "01-04-2024",
	}

	_, err := downloadMyAmeria(cfg, t.TempDir(), client)
	if err == nil {
		t.Fatal("expected error for 401")
	}

	ufe := MapError(err)
	if !strings.Contains(ufe.Message, "401") {
		t.Fatalf("MapError message = %q, want 401 mention", ufe.Message)
	}
	if ufe.Hint == "" {
		t.Fatal("expected non-empty hint for 401")
	}
}

func TestMapError_MyAmeriaInvalidJSON(t *testing.T) {
	err := fmt.Errorf("MyAmeria history response is not valid JSON (%v): %w", os.ErrInvalid, ErrMyAmeriaInvalidResponse)
	ufe := MapError(err)
	if !strings.Contains(ufe.Message, "not JSON") {
		t.Fatalf("MapError message = %q", ufe.Message)
	}
	if ufe.Hint == "" {
		t.Fatal("expected hint for invalid JSON")
	}
}
