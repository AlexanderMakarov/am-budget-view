package app

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AlexanderMakarov/am-budget-view/internal/bankdownload"
	"github.com/AlexanderMakarov/am-budget-view/internal/config"
)

const testConfigYAML = `inecobankStatementXmlFilesGlob: "*.xml"
inecobankStatementXlsxFilesGlob: "*.xlsx"
ameriaCsvFilesGlob: "*.csv"
myAmeriaAccountStatementXlsxFilesGlob: "*.xls"
myAmeriaHistoryXlsFilesGlob: "History*.xls"
genericCsvFilesGlob: "generic*.csv"
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
    authToken: "token-1"
    sinceDate: "01-01-2024"
  ameriaBusiness:
    enabled: false
    sinceDate: "01-01-2024"
`

func createTestDataHandler(t *testing.T) (*DataHandler, string) {
	t.Helper()
	tempFile, err := os.CreateTemp("", "app_bankdownload_*.yaml")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := tempFile.WriteString(testConfigYAML); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	tempFile.Close()

	cfg, err := config.ReadConfig(tempFile.Name())
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	return &DataHandler{
		ConfigPath: tempFile.Name(),
		Config:     cfg,
	}, tempFile.Name()
}

func TestUpdateBankDownloads_MergesPartialFields(t *testing.T) {
	dh, configPath := createTestDataHandler(t)
	defer os.Remove(configPath)

	err := dh.UpdateBankDownloads(config.BankDownloads{
		StaleThresholdDays: 14,
		MyAmeria: config.MyAmeriaDownloadConfig{
			AuthToken: "token-2",
		},
	})
	if err != nil {
		t.Fatalf("UpdateBankDownloads: %v", err)
	}

	if dh.Config.BankDownloads.StaleThresholdDays != 14 {
		t.Errorf("StaleThresholdDays = %d, want 14", dh.Config.BankDownloads.StaleThresholdDays)
	}
	myAmeria := dh.Config.BankDownloads.MyAmeria
	if myAmeria.ClientId != "client-1" {
		t.Errorf("ClientId = %q, want client-1", myAmeria.ClientId)
	}
	if myAmeria.AuthToken != "" {
		t.Errorf("AuthToken = %q, want empty (secrets are not persisted)", myAmeria.AuthToken)
	}
	if !myAmeria.Enabled {
		t.Error("Expected myAmeria to stay enabled")
	}

	reloaded, err := config.ReadConfig(configPath)
	if err != nil {
		t.Fatalf("ReadConfig after write: %v", err)
	}
	if reloaded.BankDownloads.MyAmeria.AuthToken != "" {
		t.Errorf("Persisted AuthToken = %q, want empty", reloaded.BankDownloads.MyAmeria.AuthToken)
	}
}

func TestUpdateBankDownloads_ValidationError(t *testing.T) {
	dh, configPath := createTestDataHandler(t)
	defer os.Remove(configPath)

	err := dh.UpdateBankDownloads(config.BankDownloads{
		MyAmeria: config.MyAmeriaDownloadConfig{
			SinceDate: "2024-01-01",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "must be in DD-MM-YYYY format") {
		t.Fatalf("Expected sinceDate validation error, got %v", err)
	}
}

func TestDownloadBank_UnsupportedSource(t *testing.T) {
	dh, configPath := createTestDataHandler(t)
	defer os.Remove(configPath)

	_, err := dh.DownloadBank("inecobank_xml", nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported bank download source") {
		t.Fatalf("Expected unsupported source error, got %v", err)
	}
}

func TestDownloadBank_RecordsErrorMetadata(t *testing.T) {
	dh, configPath := createTestDataHandler(t)
	defer os.Remove(configPath)

	origMyAmeria := downloadMyAmeriaFn
	downloadMyAmeriaFn = func(cfg config.MyAmeriaDownloadConfig, wd string) (string, error) {
		return "", errTestDownload
	}
	t.Cleanup(func() { downloadMyAmeriaFn = origMyAmeria })

	before := time.Now().UTC()
	_, err := dh.DownloadBank(string(bankdownload.SourceMyAmeriaGeneric), nil)
	if err == nil {
		t.Fatal("Expected download error")
	}

	myAmeria := dh.Config.BankDownloads.MyAmeria
	if myAmeria.LastDownloadStatus != "error" {
		t.Errorf("LastDownloadStatus = %q, want error", myAmeria.LastDownloadStatus)
	}
	if myAmeria.LastDownloadError != errTestDownload.Error() {
		t.Errorf("LastDownloadError = %q, want %q", myAmeria.LastDownloadError, errTestDownload.Error())
	}
	if myAmeria.LastDownloadAt == "" {
		t.Fatal("Expected LastDownloadAt to be set")
	}
	lastAt, parseErr := time.Parse(time.RFC3339, myAmeria.LastDownloadAt)
	if parseErr != nil {
		t.Fatalf("Parse LastDownloadAt: %v", parseErr)
	}
	if lastAt.Before(before.Add(-time.Minute)) {
		t.Errorf("LastDownloadAt %v is too old", lastAt)
	}

	reloaded, readErr := config.ReadConfig(configPath)
	if readErr != nil {
		t.Fatalf("ReadConfig after download: %v", readErr)
	}
	if reloaded.BankDownloads.MyAmeria.LastDownloadStatus != "error" {
		t.Errorf("Persisted LastDownloadStatus = %q, want error", reloaded.BankDownloads.MyAmeria.LastDownloadStatus)
	}
}

func TestDownloadBank_RecordsSuccessMetadata(t *testing.T) {
	dh, configPath := createTestDataHandler(t)
	defer os.Remove(configPath)

	origMyAmeria := downloadMyAmeriaFn
	origRebuild := rebuildFromFilesFn
	downloadMyAmeriaFn = func(cfg config.MyAmeriaDownloadConfig, wd string) (string, error) {
		return "generic.csv", nil
	}
	rebuildFromFilesFn = func(dh *DataHandler) error { return nil }
	t.Cleanup(func() {
		downloadMyAmeriaFn = origMyAmeria
		rebuildFromFilesFn = origRebuild
	})

	paths, err := dh.DownloadBank(string(bankdownload.SourceMyAmeriaGeneric), nil)
	if err != nil {
		t.Fatalf("DownloadBank: %v", err)
	}
	if len(paths) != 1 || paths[0] != "generic.csv" {
		t.Errorf("paths = %v, want [generic.csv]", paths)
	}

	myAmeria := dh.Config.BankDownloads.MyAmeria
	if myAmeria.LastDownloadStatus != "ok" {
		t.Errorf("LastDownloadStatus = %q, want ok", myAmeria.LastDownloadStatus)
	}
	if myAmeria.LastDownloadError != "" {
		t.Errorf("LastDownloadError = %q, want empty", myAmeria.LastDownloadError)
	}
}

func TestGetSourceHealth_DelegatesToBankdownload(t *testing.T) {
	dh, configPath := createTestDataHandler(t)
	defer os.Remove(configPath)

	health := dh.GetSourceHealth()
	if len(health) != len(bankdownload.AllSources) {
		t.Fatalf("Expected %d health rows, got %d", len(bankdownload.AllSources), len(health))
	}
	if health[0].SourceID != bankdownload.SourceInecobankXML {
		t.Errorf("First source = %q, want %q", health[0].SourceID, bankdownload.SourceInecobankXML)
	}
}

var errTestDownload = errTestDownloadType("simulated download failure")

type errTestDownloadType string

func (e errTestDownloadType) Error() string { return string(e) }
