package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/AlexanderMakarov/am-budget-view/internal/app"
	"github.com/AlexanderMakarov/am-budget-view/internal/config"
)

const testUIConfigYAML = `inecobankStatementXmlFilesGlob: "*.xml"
genericCsvFilesGlob: "generic*.csv"
ameriaCsvFilesGlob: "*.csv"
groupAllUnknownTransactions: true
groups:
  g1:
    substrings:
      - Sub1
bankDownloads:
  staleThresholdDays: 7
  myAmeria:
    enabled: true
    clientId: "client-1"
    sinceDate: "01-01-2024"
  ameriaBusiness:
    enabled: false
    sinceDate: "01-01-2024"
`

func newTestDataHandler(t *testing.T) (*app.DataHandler, string) {
	t.Helper()
	tempFile, err := os.CreateTemp(t.TempDir(), "ui_bankdownload_*.yaml")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := tempFile.WriteString(testUIConfigYAML); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	tempFile.Close()

	cfg, err := config.ReadConfig(tempFile.Name())
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	return &app.DataHandler{ConfigPath: tempFile.Name(), Config: cfg}, tempFile.Name()
}

func decodeBankDownloadResp(t *testing.T, rec *httptest.ResponseRecorder) bankDownloadJSONResponse {
	t.Helper()
	var resp bankDownloadJSONResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return resp
}

func TestHandleBankDownloadsConfig_MethodNotAllowed(t *testing.T) {
	dh, _ := newTestDataHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/bank-downloads/config", nil)

	handleBankDownloadsConfig(dh)(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleBankDownloadsConfig_InvalidJSON(t *testing.T) {
	dh, _ := newTestDataHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bank-downloads/config", strings.NewReader("{not json"))

	handleBankDownloadsConfig(dh)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	resp := decodeBankDownloadResp(t, rec)
	if resp.OK || resp.Error != "Invalid JSON body" {
		t.Fatalf("resp = %+v, want OK=false Error=Invalid JSON body", resp)
	}
}

func TestHandleBankDownloadsConfig_ValidationError(t *testing.T) {
	dh, _ := newTestDataHandler(t)
	rec := httptest.NewRecorder()
	// Wrong sinceDate format (ISO instead of DD-MM-YYYY) must be rejected.
	body := `{"bankDownloads":{"myAmeria":{"sinceDate":"2024-01-01"}}}`
	req := httptest.NewRequest(http.MethodPost, "/bank-downloads/config", strings.NewReader(body))

	handleBankDownloadsConfig(dh)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	resp := decodeBankDownloadResp(t, rec)
	if resp.OK || !strings.Contains(resp.Error, "DD-MM-YYYY") {
		t.Fatalf("resp = %+v, want validation error mentioning DD-MM-YYYY", resp)
	}
}

func TestHandleBankDownloadsConfig_SuccessDoesNotPersistSecrets(t *testing.T) {
	dh, configPath := newTestDataHandler(t)
	rec := httptest.NewRecorder()
	body := `{"bankDownloads":{"staleThresholdDays":14,"myAmeria":{"clientId":"client-2","authToken":"secret-token","sinceDate":"02-02-2024"}}}`
	req := httptest.NewRequest(http.MethodPost, "/bank-downloads/config", strings.NewReader(body))

	handleBankDownloadsConfig(dh)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want %d", rec.Code, rec.Body.String(), http.StatusOK)
	}
	if resp := decodeBankDownloadResp(t, rec); !resp.OK {
		t.Fatalf("resp = %+v, want OK=true", resp)
	}

	// In-memory config reflects the merge but never the secret.
	my := dh.Config.BankDownloads.MyAmeria
	if my.ClientId != "client-2" || my.SinceDate != "02-02-2024" {
		t.Errorf("merged config = %+v, want clientId=client-2 sinceDate=02-02-2024", my)
	}
	if my.AuthToken != "" {
		t.Errorf("AuthToken = %q, want empty (secrets must not be persisted)", my.AuthToken)
	}
	if dh.Config.BankDownloads.StaleThresholdDays != 14 {
		t.Errorf("StaleThresholdDays = %d, want 14", dh.Config.BankDownloads.StaleThresholdDays)
	}

	reloaded, err := config.ReadConfig(configPath)
	if err != nil {
		t.Fatalf("ReadConfig after write: %v", err)
	}
	if reloaded.BankDownloads.MyAmeria.AuthToken != "" {
		t.Errorf("persisted AuthToken = %q, want empty", reloaded.BankDownloads.MyAmeria.AuthToken)
	}
	if reloaded.BankDownloads.MyAmeria.ClientId != "client-2" {
		t.Errorf("persisted ClientId = %q, want client-2", reloaded.BankDownloads.MyAmeria.ClientId)
	}
}

func TestHandleBankDownloadsRun_MethodNotAllowed(t *testing.T) {
	dh, _ := newTestDataHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/bank-downloads/run", nil)

	handleBankDownloadsRun(dh)(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleBankDownloadsRun_InvalidJSON(t *testing.T) {
	dh, _ := newTestDataHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bank-downloads/run", strings.NewReader("nope"))

	handleBankDownloadsRun(dh)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if resp := decodeBankDownloadResp(t, rec); resp.OK || resp.Error != "Invalid JSON body" {
		t.Fatalf("resp = %+v, want Invalid JSON body", resp)
	}
}

func TestHandleBankDownloadsRun_MissingSourceID(t *testing.T) {
	dh, _ := newTestDataHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bank-downloads/run", strings.NewReader(`{}`))

	handleBankDownloadsRun(dh)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if resp := decodeBankDownloadResp(t, rec); resp.OK || resp.Error != "sourceId is required" {
		t.Fatalf("resp = %+v, want sourceId is required", resp)
	}
}

func TestHandleBankDownloadsRun_UnsupportedSource(t *testing.T) {
	dh, _ := newTestDataHandler(t)
	rec := httptest.NewRecorder()
	// A real source id that does not support in-app download must be rejected before any network call.
	req := httptest.NewRequest(http.MethodPost, "/bank-downloads/run", strings.NewReader(`{"sourceId":"inecobank_xml"}`))

	handleBankDownloadsRun(dh)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	resp := decodeBankDownloadResp(t, rec)
	if resp.OK || !strings.Contains(resp.Error, "unsupported bank download source") {
		t.Fatalf("resp = %+v, want unsupported source error", resp)
	}
}

func TestHandleBankDoc_InvalidPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"empty source id", "/docs/bank/"},
		{"nested path", "/docs/bank/foo/bar"},
		{"unknown source id", "/docs/bank/not-a-bank"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)

			handleBankDoc()(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
		})
	}
}
