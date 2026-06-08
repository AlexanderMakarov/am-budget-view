package bankdownload

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/AlexanderMakarov/am-budget-view/internal/config"
)

func testAmeriaBusinessCookie(session string) string {
	return "TS0180e8bb=abc; AccessToken=access; RefreshToken=" + session
}

func TestMapError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		wantMsg  string
		wantHint string
	}{
		{
			name:     "nil",
			err:      nil,
			wantMsg:  "",
			wantHint: "",
		},
		{
			name:     "unauthorized",
			err:      ErrUnauthorized,
			wantMsg:  "AmeriaBank Business API returned 401 Unauthorized",
			wantHint: "session cookie may have expired",
		},
		{
			name:     "refresh unauthorized",
			err:      ErrRefreshUnauthorized,
			wantMsg:  "AmeriaBank Business session refresh failed (401)",
			wantHint: "RefreshToken expired",
		},
		{
			name:     "refresh failed",
			err:      ErrRefreshFailed,
			wantMsg:  "AmeriaBank Business session refresh failed",
			wantHint: "network connection",
		},
		{
			name:     "ameria business 401",
			err:      &ameriaBusinessHTTPError{statusCode: 401, op: "Accounts", body: "account 4017... denied"},
			wantMsg:  "AmeriaBank Business API returned 401 Unauthorized",
			wantHint: "business.myameria.am",
		},
		{
			name:     "ameria business 503",
			err:      &ameriaBusinessHTTPError{statusCode: 503, op: "Export", body: "service unavailable"},
			wantMsg:  "AmeriaBank Business server error",
			wantHint: "temporarily unavailable",
		},
		{
			name:     "ameria business 403",
			err:      &ameriaBusinessHTTPError{statusCode: 403, op: "Accounts", body: "forbidden"},
			wantMsg:  "AmeriaBank Business API returned 403 Forbidden",
			wantHint: "access to this resource",
		},
		{
			name:     "ameria business other status falls through to raw error",
			err:      &ameriaBusinessHTTPError{statusCode: 418, op: "Export", body: "teapot"},
			wantMsg:  "AmeriaBank Business Export request failed: HTTP 418: teapot",
			wantHint: "application logs",
		},
		{
			name:     "generic",
			err:      fmt.Errorf("something unexpected"),
			wantMsg:  "something unexpected",
			wantHint: "application logs",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MapError(tt.err)
			if tt.err == nil {
				if got.Message != "" || got.Hint != "" {
					t.Fatalf("expected empty UserFacingError, got %+v", got)
				}
				return
			}
			if got.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", got.Message, tt.wantMsg)
			}
			if !strings.Contains(strings.ToLower(got.Hint), strings.ToLower(tt.wantHint)) {
				t.Errorf("Hint = %q, want substring %q", got.Hint, tt.wantHint)
			}
		})
	}
}

func TestDownloadAmeriaBusiness_Success(t *testing.T) {
	t.Parallel()

	var refreshCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/Authentication/Refresh"):
			refreshCalls.Add(1)
			http.SetCookie(w, &http.Cookie{Name: "AccessToken", Value: "new-access"})
			http.SetCookie(w, &http.Cookie{Name: "RefreshToken", Value: "new-refresh"})
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/Accounts"):
			accountType := r.URL.Query().Get("accountType")
			var payload []map[string]any
			switch accountType {
			case accountTypeSettlement:
				payload = []map[string]any{
					{
						"id": 101, "number": "1234567890123456", "currency": "AMD",
						"name": "Main Settlement", "subType": accountTypeSettlement,
						"balance": 1000.5, "accountStatus": "Open",
					},
				}
			case accountTypeCard:
				payload = []map[string]any{
					{
						"id": 202, "number": "9876543210987654", "currency": "USD",
						"name": "Corp Card", "subType": accountTypeCard,
						"balance": 250.0, "accountStatus": "Open",
					},
				}
			default:
				t.Errorf("unexpected accountType %q", accountType)
				http.Error(w, "bad account type", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/Statements/Export"):
			if got := r.Header.Get("Cookie"); !strings.Contains(got, "RefreshToken=abc") && !strings.Contains(got, "AccessToken=new-access") {
				t.Errorf("unexpected Cookie header: %q", got)
			}
			if r.URL.Query().Get("exportFormat") != "Csv" {
				t.Errorf("exportFormat = %q, want Csv", r.URL.Query().Get("exportFormat"))
			}
			if r.URL.Query().Get("startDate") != "2024-01-01" {
				t.Errorf("startDate = %q, want 2024-01-01", r.URL.Query().Get("startDate"))
			}
			if r.URL.Query().Get("withAmd") != "true" {
				t.Errorf("withAmd = %q, want true", r.URL.Query().Get("withAmd"))
			}
			w.Header().Set("Content-Type", "text/csv")
			_, _ = w.Write([]byte("Date,Doc.No.,Type,Account,Details,Debit,Credit,Remitter/Beneficiary\n01/01/2024,1,Transfer,123,,10.00,,Someone\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	wd := t.TempDir()
	client := newAmeriaBusinessClient(server.Client())
	client.apiBase = server.URL + "/api/v1"
	client.origin = ameriaBusinessOrigin

	cfg := config.AmeriaBusinessDownloadConfig{
		Enabled:   true,
		Cookie:    testAmeriaBusinessCookie("abc"),
		SinceDate: "01-01-2024",
	}

	paths, err := downloadAmeriaBusiness(cfg, wd, client)
	if err != nil {
		t.Fatalf("downloadAmeriaBusiness() error = %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d: %v", len(paths), paths)
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("ReadFile(%q): %v", p, err)
		}
		if !strings.Contains(string(data), "Date,Doc.No.") {
			t.Errorf("file %q missing CSV header: %q", p, string(data))
		}
	}

	settlementPath := filepath.Join(wd, "AccountStatement 1234567890123456 Main Settlement since 01-01-2024.csv")
	cardPath := filepath.Join(wd, "AccountStatement 9876543210987654 Corp Card since 01-01-2024.csv")
	if paths[0] != settlementPath || paths[1] != cardPath {
		t.Errorf("paths = %v, want [%q, %q]", paths, settlementPath, cardPath)
	}
}

func TestDownloadAmeriaBusiness_401RetryWithRefresh(t *testing.T) {
	t.Parallel()

	var settlementCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/Authentication/Refresh"):
			http.SetCookie(w, &http.Cookie{Name: "AccessToken", Value: "fresh"})
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/Accounts"):
			accountType := r.URL.Query().Get("accountType")
			if accountType != accountTypeSettlement {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode([]map[string]any{})
				return
			}
			if settlementCalls.Add(1) == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"expired"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 55, "number": "111", "currency": "AMD", "name": "Retry Account", "subType": accountTypeSettlement},
			})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/Statements/Export"):
			w.Header().Set("Content-Type", "text/csv")
			_, _ = w.Write([]byte("Date,Doc.No.\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	wd := t.TempDir()
	client := newAmeriaBusinessClient(server.Client())
	client.apiBase = server.URL + "/api/v1"

	cfg := config.AmeriaBusinessDownloadConfig{
		Enabled:   true,
		Cookie:    testAmeriaBusinessCookie("old"),
		SinceDate: "01-01-2024",
	}

	paths, err := downloadAmeriaBusiness(cfg, wd, client)
	if err != nil {
		t.Fatalf("downloadAmeriaBusiness() error = %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if settlementCalls.Load() != 2 {
		t.Errorf("settlement account calls = %d, want 2 (401 then retry)", settlementCalls.Load())
	}
}

func TestRefreshCookie_401(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid refresh"}`))
	}))
	defer server.Close()

	client := newAmeriaBusinessClient(server.Client())
	client.apiBase = server.URL + "/api/v1"

	_, err := client.refreshCookie("session=old")
	if !errors.Is(err, ErrRefreshUnauthorized) {
		t.Fatalf("refreshCookie() error = %v, want ErrRefreshUnauthorized", err)
	}
	ufe := MapError(err)
	if !strings.Contains(ufe.Hint, "RefreshToken expired") {
		t.Errorf("Hint = %q, want RefreshToken hint", ufe.Hint)
	}
}

func TestDownloadStatementCSV_JSONWrapped(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"data": "Date,Amount\n01/01/2024,10.00\n",
		})
	}))
	defer server.Close()

	wd := t.TempDir()
	outPath := filepath.Join(wd, "wrapped.csv")

	client := newAmeriaBusinessClient(server.Client())
	client.apiBase = server.URL + "/api/v1"

	if err := client.downloadStatementCSV("session=abc", 1, "2024-01-01", "2024-06-01", outPath); err != nil {
		t.Fatalf("downloadStatementCSV() error = %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "Date,Amount\n01/01/2024,10.00\n" {
		t.Errorf("file content = %q", string(data))
	}
}

func TestNormalizeAmeriaBusinessCookie(t *testing.T) {
	t.Parallel()

	got := normalizeAmeriaBusinessCookie("  TS0180e8bb=abc;\nRefreshToken=xyz\r\n  ")
	if got != "TS0180e8bb=abc;RefreshToken=xyz" {
		t.Errorf("normalizeAmeriaBusinessCookie() = %q", got)
	}
}

func TestValidateAmeriaBusinessCookie_Incomplete(t *testing.T) {
	t.Parallel()

	// ~269 chars: only TS tracking cookies, no JWT tokens (matches failed download logs).
	cookie := strings.Repeat("TS0180e8bb=", 1) + strings.Repeat("a", 120) + "; " +
		strings.Repeat("TS010d720a=", 1) + strings.Repeat("b", 120)
	err := validateAmeriaBusinessCookie(cookie)
	if !errors.Is(err, ErrIncompleteCookie) {
		t.Fatalf("validateAmeriaBusinessCookie() error = %v, want ErrIncompleteCookie", err)
	}
	ufe := MapError(err)
	if !strings.Contains(ufe.Hint, "RefreshToken") {
		t.Errorf("Hint = %q, want RefreshToken guidance", ufe.Hint)
	}
}

func TestValidateAmeriaBusinessCookie_OK(t *testing.T) {
	t.Parallel()

	err := validateAmeriaBusinessCookie("TS0180e8bb=abc; AccessToken=jwt; RefreshToken=refresh")
	if err != nil {
		t.Fatalf("validateAmeriaBusinessCookie() error = %v, want nil", err)
	}
}

func TestResolveOutputDir_OutsideWorkingDirectoryFallsBack(t *testing.T) {
	t.Parallel()

	wd := t.TempDir()
	got := resolveOutputDir(wd, "..")
	want, err := filepath.Abs(wd)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	if got != want {
		t.Errorf("resolveOutputDir(wd, ..) = %q, want %q", got, want)
	}
}

func TestResolveOutputDir_Subfolder(t *testing.T) {
	t.Parallel()

	wd := t.TempDir()
	got := resolveOutputDir(wd, "statements")
	want := filepath.Join(wd, "statements")
	if got != want {
		t.Errorf("resolveOutputDir() = %q, want %q", got, want)
	}
}

func TestLatin1Safe(t *testing.T) {
	t.Parallel()

	got := latin1Safe("session=abc\u0100def")
	if got != "session=abcdef" {
		t.Errorf("latin1Safe() = %q, want %q", got, "session=abcdef")
	}
}

func TestAmeriaBusinessHeaders(t *testing.T) {
	t.Parallel()

	h := ameriaBusinessHeaders("session=abc", ameriaBusinessOrigin)
	if h.Get("Origin") != ameriaBusinessOrigin {
		t.Errorf("Origin = %q", h.Get("Origin"))
	}
	if h.Get("DeviceId") != "MyAmeriaBusiness Web" {
		t.Errorf("DeviceId = %q", h.Get("DeviceId"))
	}
	if h.Get("Cookie") != "session=abc" {
		t.Errorf("Cookie = %q", h.Get("Cookie"))
	}
}

func TestDownloadAmeriaBusiness_MissingCookie(t *testing.T) {
	t.Parallel()

	_, err := DownloadAmeriaBusiness(config.AmeriaBusinessDownloadConfig{Enabled: false}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "cookie is required") {
		t.Fatalf("expected missing cookie error, got %v", err)
	}
}

func TestDownloadAmeriaBusiness_OutputFolder(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/Accounts"):
			w.Header().Set("Content-Type", "application/json")
			accountType := r.URL.Query().Get("accountType")
			if accountType == accountTypeSettlement {
				_ = json.NewEncoder(w).Encode([]map[string]any{
					{"id": 1, "number": "111", "currency": "AMD", "name": "A"},
				})
			} else {
				_ = json.NewEncoder(w).Encode([]map[string]any{})
			}
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/Statements/Export"):
			_, _ = w.Write([]byte("csv"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	wd := t.TempDir()
	subDir := filepath.Join(wd, "statements")
	client := newAmeriaBusinessClient(server.Client())
	client.apiBase = server.URL + "/api/v1"

	cfg := config.AmeriaBusinessDownloadConfig{
		Enabled:      true,
		Cookie:       testAmeriaBusinessCookie("abc"),
		SinceDate:    "01-01-2024",
		OutputFolder: "statements",
	}

	paths, err := downloadAmeriaBusiness(cfg, wd, client)
	if err != nil {
		t.Fatalf("downloadAmeriaBusiness() error = %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	want := filepath.Join(subDir, "AccountStatement 111 A since 01-01-2024.csv")
	if paths[0] != want {
		t.Errorf("path = %q, want %q", paths[0], want)
	}
}
