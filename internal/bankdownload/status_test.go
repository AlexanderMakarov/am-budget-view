package bankdownload

import (
	"testing"
	"time"

	"github.com/AlexanderMakarov/am-budget-view/internal/config"
	"github.com/AlexanderMakarov/am-budget-view/internal/model"
)

func testSource(tag string) *model.TransactionsSource {
	return &model.TransactionsSource{Tag: tag}
}

func fileInfo(tag string, from, to time.Time) model.FileInfo {
	return model.FileInfo{
		Path:         "test-" + tag,
		Source:       testSource(tag),
		FromDate:     from,
		ToDate:       to,
		ModifiedTime: to,
	}
}

func TestMatchFileInfo_TagPrefixes(t *testing.T) {
	cases := []struct {
		tag    string
		wantID SourceID
	}{
		{"InecoXml:AMD", SourceInecobankXML},
		{"InecoExcelRegular:AMD", SourceInecobankXLSX},
		{"InecoExcelCard:USD", SourceInecobankXLSX},
		{"AmeriaCsv:AMD", SourceAmeriaBusiness},
		{"MyAmeriaXls:AMD", SourceMyAmeriaHistory},
		{"GenericCsv:USD", SourceMyAmeriaGeneric},
		{"ArdshinXlsx:AMD", SourceArdshin},
		{"AcbaAccountExcel:AMD", SourceAcbaAccount},
		{"AcbaCardExcel:AMD", SourceAcbaCard},
		{"Unknown:AMD", ""},
	}

	for _, tc := range cases {
		got := MatchFileInfo(fileInfo(tc.tag, time.Time{}, time.Time{}))
		if got != tc.wantID {
			t.Errorf("MatchFileInfo(%q) = %q, want %q", tc.tag, got, tc.wantID)
		}
	}
}

func TestAllSources_RegistryCompleteness(t *testing.T) {
	if len(AllSources) != 8 {
		t.Fatalf("expected 8 sources, got %d", len(AllSources))
	}

	cfg := &config.Config{
		InecobankStatementXmlFilesGlob:       "Statement*.xml",
		InecobankStatementXlsxFilesGlob:      "statement*.xlsx",
		AmeriaCsvFilesGlob:                   "AccountStatement*.csv",
		MyAmeriaAccountStatementXlsFilesGlob: "* account statement *.xls",
		MyAmeriaHistoryXlsFilesGlob:          "History *.xls",
		GenericCsvFilesGlob:                  "generic*.csv",
		ArdshinbankXlsxFilesGlob:             "STATEMENT_*.xlsx",
		AcbaRegularAccountXlsFilesGlob:       "AccountStatement*.xls",
		AcbaCardXlsFilesGlob:                 "CardStatement*.xls",
	}

	for _, def := range AllSources {
		if def.Glob(cfg) == "" {
			t.Errorf("source %q has empty glob", def.ID)
		}
		if def.DocSourceID == "" {
			t.Errorf("source %q has empty DocSourceID", def.ID)
		}
	}

	if !AllSources[4].SupportsInAppDownload || !AllSources[4].SupportsCliDownload {
		t.Error("my_ameria_generic should support in-app and CLI download")
	}
	if !AllSources[2].SupportsInAppDownload || !AllSources[2].SupportsCliDownload {
		t.Error("ameria_business should support in-app and CLI download")
	}
}

func TestComputeSourceHealth_OK(t *testing.T) {
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	latest := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	files := []model.FileInfo{
		fileInfo("InecoXml:AMD", latest.AddDate(0, 0, -5), latest),
		fileInfo("AmeriaCsv:AMD", latest.AddDate(0, 0, -2), latest),
	}

	cfg := &config.Config{BankDownloads: config.BankDownloads{StaleThresholdDays: 7}}
	health := computeSourceHealthAt(files, cfg, now)

	byID := indexHealth(health)
	if byID[SourceInecobankXML].Status != StatusOK {
		t.Errorf("inecobank_xml status = %q, want ok", byID[SourceInecobankXML].Status)
	}
	if byID[SourceInecobankXML].FilesCount != 1 {
		t.Errorf("inecobank_xml FilesCount = %d, want 1", byID[SourceInecobankXML].FilesCount)
	}
	if !byID[SourceInecobankXML].LatestTxDate.Equal(latest) {
		t.Errorf("inecobank_xml LatestTxDate = %v, want %v", byID[SourceInecobankXML].LatestTxDate, latest)
	}
	if byID[SourceAcbaCard].Status != StatusNoFiles {
		t.Errorf("acba_card status = %q, want no_files", byID[SourceAcbaCard].Status)
	}
}

func TestComputeSourceHealth_Stale(t *testing.T) {
	now := time.Date(2024, 3, 25, 12, 0, 0, 0, time.UTC)
	globalLatest := time.Date(2024, 3, 20, 0, 0, 0, 0, time.UTC)
	staleLatest := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)

	files := []model.FileInfo{
		fileInfo("InecoXml:AMD", globalLatest.AddDate(0, 0, -1), globalLatest),
		fileInfo("ArdshinXlsx:AMD", staleLatest.AddDate(0, 0, -30), staleLatest),
	}

	cfg := &config.Config{BankDownloads: config.BankDownloads{StaleThresholdDays: 7}}
	health := computeSourceHealthAt(files, cfg, now)

	byID := indexHealth(health)
	if byID[SourceArdshin].Status != StatusStale {
		t.Errorf("ardshin status = %q, want stale; detail: %s", byID[SourceArdshin].Status, byID[SourceArdshin].StatusDetail)
	}
	if byID[SourceInecobankXML].Status != StatusOK {
		t.Errorf("inecobank_xml status = %q, want ok", byID[SourceInecobankXML].Status)
	}
}

func TestComputeSourceHealth_NoFiles(t *testing.T) {
	cfg := &config.Config{BankDownloads: config.BankDownloads{StaleThresholdDays: 7}}
	health := ComputeSourceHealth(nil, cfg)

	if len(health) != 8 {
		t.Fatalf("expected 8 health rows, got %d", len(health))
	}
	for _, h := range health {
		if h.Status != StatusNoFiles {
			t.Errorf("source %q with no files: status = %q, want no_files", h.SourceID, h.Status)
		}
	}
}

func TestComputeSourceHealth_DownloadError(t *testing.T) {
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	latest := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	files := []model.FileInfo{
		fileInfo("AmeriaCsv:AMD", latest.AddDate(0, 0, -1), latest),
	}

	cfg := &config.Config{
		BankDownloads: config.BankDownloads{
			StaleThresholdDays: 7,
			AmeriaBusiness: config.AmeriaBusinessDownloadConfig{
				Enabled:            true,
				LastDownloadStatus: StatusError,
				LastDownloadError:  "HTTP 401",
			},
		},
	}

	byID := indexHealth(computeSourceHealthAt(files, cfg, now))
	h := byID[SourceAmeriaBusiness]
	if h.Status != StatusError {
		t.Errorf("ameria_business status = %q, want error", h.Status)
	}
	if h.DownloadConfigured != true {
		t.Error("expected DownloadConfigured true")
	}
	if h.LastDownloadError != "HTTP 401" {
		t.Errorf("LastDownloadError = %q, want HTTP 401", h.LastDownloadError)
	}
}

func TestComputeSourceHealth_ManualOnly(t *testing.T) {
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	latest := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	files := []model.FileInfo{
		fileInfo("GenericCsv:USD", latest.AddDate(0, 0, -1), latest),
	}

	cfg := &config.Config{
		BankDownloads: config.BankDownloads{
			StaleThresholdDays: 7,
			MyAmeria: config.MyAmeriaDownloadConfig{
				Enabled: false,
			},
		},
	}

	byID := indexHealth(computeSourceHealthAt(files, cfg, now))
	h := byID[SourceMyAmeriaGeneric]
	if h.Status != StatusManualOnly {
		t.Errorf("my_ameria_generic status = %q, want manual_only", h.Status)
	}
}

func TestComputeSourceHealth_DownloadErrorWithoutFiles(t *testing.T) {
	cfg := &config.Config{
		BankDownloads: config.BankDownloads{
			StaleThresholdDays: 7,
			MyAmeria: config.MyAmeriaDownloadConfig{
				Enabled:            true,
				LastDownloadStatus: StatusError,
				LastDownloadError:  "timeout",
			},
		},
	}

	byID := indexHealth(ComputeSourceHealth(nil, cfg))
	h := byID[SourceMyAmeriaGeneric]
	if h.Status != StatusError {
		t.Errorf("my_ameria_generic with failed download and no files: status = %q, want error", h.Status)
	}
}

func TestComputeSourceHealth_StaleByAgeFromToday(t *testing.T) {
	now := time.Date(2024, 4, 15, 12, 0, 0, 0, time.UTC)
	oldLatest := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	files := []model.FileInfo{
		fileInfo("InecoXml:AMD", oldLatest.AddDate(0, 0, -5), oldLatest),
	}

	cfg := &config.Config{BankDownloads: config.BankDownloads{StaleThresholdDays: 7}}
	byID := indexHealth(computeSourceHealthAt(files, cfg, now))
	if byID[SourceInecobankXML].Status != StatusStale {
		t.Errorf("single source with month-old tx: status = %q, want stale; detail: %s",
			byID[SourceInecobankXML].Status, byID[SourceInecobankXML].StatusDetail)
	}
}

func TestComputeSourceHealth_DefaultStaleThreshold(t *testing.T) {
	now := time.Date(2024, 3, 25, 12, 0, 0, 0, time.UTC)
	globalLatest := time.Date(2024, 3, 20, 0, 0, 0, 0, time.UTC)
	staleLatest := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)

	files := []model.FileInfo{
		fileInfo("InecoXml:AMD", globalLatest, globalLatest),
		fileInfo("AcbaCardExcel:AMD", staleLatest, staleLatest),
	}

	cfg := &config.Config{BankDownloads: config.BankDownloads{}}
	byID := indexHealth(computeSourceHealthAt(files, cfg, now))
	if byID[SourceAcbaCard].Status != StatusStale {
		t.Errorf("acba_card with default threshold: status = %q, want stale", byID[SourceAcbaCard].Status)
	}
}

func TestBuildFileGroups_AllSourcesInOrder(t *testing.T) {
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	latest := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)
	files := []model.FileInfo{
		fileInfo("InecoXml:AMD", latest.AddDate(0, 0, -5), latest),
		fileInfo("AmeriaCsv:AMD", latest.AddDate(0, 0, -2), latest),
	}
	cfg := &config.Config{BankDownloads: config.BankDownloads{StaleThresholdDays: 7}}

	groups := buildFileGroupsAt(files, cfg, now)
	if len(groups.Groups) != len(AllSources) {
		t.Fatalf("Groups = %d, want %d", len(groups.Groups), len(AllSources))
	}
	if !groups.Groups[0].HasFiles || groups.Groups[0].Health.SourceID != SourceInecobankXML {
		t.Errorf("first group = %+v, want inecobank_xml with files", groups.Groups[0].Health.SourceID)
	}
	if len(groups.Groups[0].Files) != 1 {
		t.Errorf("inecobank group files = %d, want 1", len(groups.Groups[0].Files))
	}
	emptyCount := 0
	for _, g := range groups.Groups {
		if !g.HasFiles {
			emptyCount++
		}
	}
	if emptyCount != len(AllSources)-2 {
		t.Errorf("empty groups = %d, want %d", emptyCount, len(AllSources)-2)
	}
}

func indexHealth(health []SourceHealth) map[SourceID]SourceHealth {
	m := make(map[SourceID]SourceHealth, len(health))
	for _, h := range health {
		m[h.SourceID] = h
	}
	return m
}
