package bankdownload

import (
	"fmt"
	"time"

	"github.com/AlexanderMakarov/am-budget-view/internal/config"
	"github.com/AlexanderMakarov/am-budget-view/internal/model"
)

const (
	StatusOK         = "ok"
	StatusStale      = "stale"
	StatusNoFiles    = "no_files"
	StatusError      = "error"
	StatusManualOnly = "manual_only"
)

// SourceHealth summarizes freshness and download state for one bank source.
type SourceHealth struct {
	SourceID              SourceID
	Name                  string
	DocSourceID           string
	FilesCount            int
	LatestTxDate          time.Time
	EarliestTxDate        time.Time
	LastFileModified      time.Time
	DownloadConfigured    bool
	LastDownloadAt        string
	LastDownloadStatus    string
	LastDownloadError     string
	Status                string
	StatusDetail          string
	SupportsInAppDownload bool
	SupportsCliDownload   bool
}

// ComputeSourceHealth builds per-source health rows from parsed files and config.
func ComputeSourceHealth(fileInfos []model.FileInfo, cfg *config.Config) []SourceHealth {
	return computeSourceHealthAt(fileInfos, cfg, time.Now())
}

func computeSourceHealthAt(fileInfos []model.FileInfo, cfg *config.Config, now time.Time) []SourceHealth {
	bySource := groupFileInfosBySource(fileInfos)
	globalLatest := globalLatestTxDate(bySource)

	staleThresholdDays := cfg.BankDownloads.StaleThresholdDays
	if staleThresholdDays == 0 {
		staleThresholdDays = 7
	}

	result := make([]SourceHealth, 0, len(AllSources))
	for _, def := range AllSources {
		files := bySource[def.ID]
		health := buildSourceHealth(def, files, globalLatest, staleThresholdDays, cfg, now)
		result = append(result, health)
	}
	return result
}

func groupFileInfosBySource(fileInfos []model.FileInfo) map[SourceID][]model.FileInfo {
	bySource := make(map[SourceID][]model.FileInfo)
	for _, fi := range fileInfos {
		id := MatchFileInfo(fi)
		if id == "" {
			continue
		}
		bySource[id] = append(bySource[id], fi)
	}
	return bySource
}

func globalLatestTxDate(bySource map[SourceID][]model.FileInfo) time.Time {
	var latest time.Time
	for _, files := range bySource {
		for _, fi := range files {
			if fi.ToDate.IsZero() {
				continue
			}
			if latest.IsZero() || fi.ToDate.After(latest) {
				latest = fi.ToDate
			}
		}
	}
	return latest
}

func buildSourceHealth(
	def SourceDefinition,
	files []model.FileInfo,
	globalLatest time.Time,
	staleThresholdDays int,
	cfg *config.Config,
	now time.Time,
) SourceHealth {
	health := SourceHealth{
		SourceID:              def.ID,
		Name:                  def.Name,
		DocSourceID:           def.DocSourceID,
		FilesCount:            len(files),
		SupportsInAppDownload: def.SupportsInAppDownload,
		SupportsCliDownload:   def.SupportsCliDownload,
	}

	downloadConfigured, lastDownloadAt, lastDownloadStatus, lastDownloadError :=
		downloadMetadata(def.ID, cfg)
	health.DownloadConfigured = downloadConfigured
	health.LastDownloadAt = lastDownloadAt
	health.LastDownloadStatus = lastDownloadStatus
	health.LastDownloadError = lastDownloadError

	if len(files) == 0 {
		health.Status = StatusNoFiles
		health.StatusDetail = fmt.Sprintf("No files matching glob: %s", def.Glob(cfg))
		if def.SupportsInAppDownload && downloadConfigured && lastDownloadStatus == StatusError {
			health.Status = StatusError
			health.StatusDetail = downloadErrorDetail(lastDownloadError)
		}
		return health
	}

	for _, fi := range files {
		if !fi.FromDate.IsZero() && (health.EarliestTxDate.IsZero() || fi.FromDate.Before(health.EarliestTxDate)) {
			health.EarliestTxDate = fi.FromDate
		}
		if !fi.ToDate.IsZero() && (health.LatestTxDate.IsZero() || fi.ToDate.After(health.LatestTxDate)) {
			health.LatestTxDate = fi.ToDate
		}
		if fi.ModifiedTime.After(health.LastFileModified) {
			health.LastFileModified = fi.ModifiedTime
		}
	}

	if def.SupportsInAppDownload && downloadConfigured && lastDownloadStatus == StatusError {
		health.Status = StatusError
		health.StatusDetail = downloadErrorDetail(lastDownloadError)
		return health
	}

	txAgeDays := 0
	if !health.LatestTxDate.IsZero() {
		txAgeDays = daysBetween(health.LatestTxDate, now)
	}

	if isStale(health.LatestTxDate, globalLatest, staleThresholdDays) {
		lagDays := daysBetween(health.LatestTxDate, globalLatest)
		health.Status = StatusStale
		health.StatusDetail = fmt.Sprintf(
			"Latest transaction is %d days behind the newest bank in this report (threshold %d days)",
			lagDays,
			staleThresholdDays,
		)
		return health
	}

	if txAgeDays > staleThresholdDays {
		health.Status = StatusStale
		health.StatusDetail = fmt.Sprintf(
			"Latest transaction is %d days old (threshold %d days)",
			txAgeDays,
			staleThresholdDays,
		)
		return health
	}

	if def.SupportsInAppDownload && !downloadConfigured {
		health.Status = StatusManualOnly
		if txAgeDays == 0 {
			health.StatusDetail = "Files added manually; in-app download is not configured"
		} else {
			health.StatusDetail = fmt.Sprintf(
				"Files added manually; latest transaction %d days ago; in-app download is not configured",
				txAgeDays,
			)
		}
		return health
	}

	health.Status = StatusOK
	if txAgeDays == 0 {
		health.StatusDetail = fmt.Sprintf("%d file(s); latest transaction is today", health.FilesCount)
	} else if txAgeDays == 1 {
		health.StatusDetail = fmt.Sprintf("%d file(s); latest transaction was yesterday", health.FilesCount)
	} else {
		health.StatusDetail = fmt.Sprintf(
			"%d file(s); latest transaction was %d days ago",
			health.FilesCount,
			txAgeDays,
		)
	}
	return health
}

func downloadMetadata(
	id SourceID,
	cfg *config.Config,
) (configured bool, lastAt, lastStatus, lastError string) {
	switch id {
	case SourceMyAmeriaGeneric:
		c := cfg.BankDownloads.MyAmeria
		return c.Enabled, c.LastDownloadAt, c.LastDownloadStatus, c.LastDownloadError
	case SourceAmeriaBusiness:
		c := cfg.BankDownloads.AmeriaBusiness
		return c.Enabled, c.LastDownloadAt, c.LastDownloadStatus, c.LastDownloadError
	default:
		return false, "", "", ""
	}
}

func downloadErrorDetail(lastError string) string {
	if lastError == "" {
		return "Last download failed"
	}
	return "Last download failed: " + lastError
}

func isStale(sourceLatest, globalLatest time.Time, thresholdDays int) bool {
	if sourceLatest.IsZero() || globalLatest.IsZero() {
		return false
	}
	if !sourceLatest.Before(globalLatest) {
		return false
	}
	return daysBetween(sourceLatest, globalLatest) > thresholdDays
}

func daysBetween(earlier, later time.Time) int {
	e := truncateToDate(earlier)
	l := truncateToDate(later)
	return int(l.Sub(e).Hours() / 24)
}

func truncateToDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
