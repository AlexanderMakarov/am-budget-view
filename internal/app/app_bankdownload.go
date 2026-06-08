package app

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/AlexanderMakarov/am-budget-view/internal/bankdownload"
	"github.com/AlexanderMakarov/am-budget-view/internal/config"
)

var (
	downloadMyAmeriaFn       = bankdownload.DownloadMyAmeria
	downloadAmeriaBusinessFn = bankdownload.DownloadAmeriaBusiness
	rebuildFromFilesFn       = func(dh *DataHandler) error { return dh.RebuildFromFiles() }
)

// BankDownloadRunOptions carries per-run credentials and options from the download dialog.
// Secrets are used only for this download and are not written to config.yaml.
type BankDownloadRunOptions struct {
	MyAmeria       *config.MyAmeriaDownloadConfig
	AmeriaBusiness *config.AmeriaBusinessDownloadConfig
}

// UpdateBankDownloads merges patch into Config.BankDownloads, validates, and writes config.yaml.
func (dh *DataHandler) UpdateBankDownloads(patch config.BankDownloads) error {
	config.MergeBankDownloads(&dh.Config.BankDownloads, patch)
	if dh.Config.BankDownloads.MyAmeria.AuthToken != "" {
		dh.Config.BankDownloads.MyAmeria.AuthToken =
			bankdownload.NormalizeMyAmeriaAuthToken(dh.Config.BankDownloads.MyAmeria.AuthToken)
	}
	return dh.writeBankDownloadsConfig()
}

func (dh *DataHandler) writeBankDownloadsConfig() error {
	if dh.Config.BankDownloads.StaleThresholdDays == 0 {
		dh.Config.BankDownloads.StaleThresholdDays = 7
	}
	// Session secrets are ephemeral; never persist them in config.yaml.
	dh.Config.BankDownloads.MyAmeria.AuthToken = ""
	dh.Config.BankDownloads.AmeriaBusiness.Cookie = ""
	if err := config.ValidateBankDownloads(&dh.Config.BankDownloads); err != nil {
		return err
	}
	return dh.Config.WriteToFile(dh.ConfigPath)
}

// DownloadBank downloads statements for a supported source and refreshes parsed data on success.
// Returns paths of files written on success.
func (dh *DataHandler) DownloadBank(sourceID string, opts *BankDownloadRunOptions) ([]string, error) {
	id := bankdownload.SourceID(sourceID)
	switch id {
	case bankdownload.SourceMyAmeriaGeneric:
		return dh.downloadMyAmeria(opts)
	case bankdownload.SourceAmeriaBusiness:
		return dh.downloadAmeriaBusiness(opts)
	default:
		return nil, fmt.Errorf("unsupported bank download source %q", sourceID)
	}
}

func (dh *DataHandler) downloadMyAmeria(opts *BankDownloadRunOptions) ([]string, error) {
	cfg := dh.Config.BankDownloads.MyAmeria
	if opts != nil && opts.MyAmeria != nil {
		cfg = mergeMyAmeriaForRun(cfg, *opts.MyAmeria)
	}
	log.Printf("MyAmeria download requested (clientIdSet=%v authTokenSet=%v sinceDate=%q)",
		cfg.ClientId != "", cfg.AuthToken != "", cfg.SinceDate)
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("working directory: %w", err)
	}

	path, downloadErr := downloadMyAmeriaFn(cfg, wd)
	dh.recordMyAmeriaDownload(downloadErr)
	if downloadErr == nil {
		dh.persistMyAmeriaPreferences(cfg)
	}
	if writeErr := dh.Config.WriteToFile(dh.ConfigPath); writeErr != nil {
		return nil, writeErr
	}
	if downloadErr != nil {
		return nil, downloadErr
	}
	if path == "" {
		log.Printf("MyAmeria: download complete — no file written")
	} else {
		log.Printf("MyAmeria: download complete — wrote %s", path)
	}
	if err := rebuildFromFilesFn(dh); err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}
	return []string{path}, nil
}

func (dh *DataHandler) downloadAmeriaBusiness(opts *BankDownloadRunOptions) ([]string, error) {
	cfg := dh.Config.BankDownloads.AmeriaBusiness
	if opts != nil && opts.AmeriaBusiness != nil {
		cfg = mergeAmeriaBusinessForRun(cfg, *opts.AmeriaBusiness)
	}
	log.Printf("AmeriaBusiness download requested (cookieSet=%v sinceDate=%q outputFolder=%q)",
		cfg.Cookie != "", cfg.SinceDate, cfg.OutputFolder)
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("working directory: %w", err)
	}

	paths, downloadErr := downloadAmeriaBusinessFn(cfg, wd)
	dh.recordAmeriaBusinessDownload(downloadErr)
	if downloadErr == nil {
		dh.persistAmeriaBusinessPreferences(cfg)
	}
	if writeErr := dh.Config.WriteToFile(dh.ConfigPath); writeErr != nil {
		return nil, writeErr
	}
	if downloadErr != nil {
		return paths, downloadErr
	}
	log.Printf("AmeriaBusiness: download complete — wrote %d file(s): %s",
		len(paths), strings.Join(paths, ", "))
	if err := rebuildFromFilesFn(dh); err != nil {
		return paths, err
	}
	return paths, nil
}

func mergeMyAmeriaForRun(base, override config.MyAmeriaDownloadConfig) config.MyAmeriaDownloadConfig {
	out := base
	if override.ClientId != "" {
		out.ClientId = override.ClientId
	}
	if override.AuthToken != "" {
		out.AuthToken = bankdownload.NormalizeMyAmeriaAuthToken(override.AuthToken)
	}
	if override.SinceDate != "" {
		out.SinceDate = override.SinceDate
	}
	return out
}

func mergeAmeriaBusinessForRun(base, override config.AmeriaBusinessDownloadConfig) config.AmeriaBusinessDownloadConfig {
	out := base
	if override.Cookie != "" {
		out.Cookie = override.Cookie
	}
	if override.SinceDate != "" {
		out.SinceDate = override.SinceDate
	}
	if override.OutputFolder != "" {
		out.OutputFolder = override.OutputFolder
	}
	return out
}

func (dh *DataHandler) persistMyAmeriaPreferences(cfg config.MyAmeriaDownloadConfig) {
	if cfg.ClientId != "" {
		dh.Config.BankDownloads.MyAmeria.ClientId = cfg.ClientId
	}
	if cfg.SinceDate != "" {
		dh.Config.BankDownloads.MyAmeria.SinceDate = cfg.SinceDate
	}
	dh.Config.BankDownloads.MyAmeria.AuthToken = ""
}

func (dh *DataHandler) persistAmeriaBusinessPreferences(cfg config.AmeriaBusinessDownloadConfig) {
	if cfg.SinceDate != "" {
		dh.Config.BankDownloads.AmeriaBusiness.SinceDate = cfg.SinceDate
	}
	dh.Config.BankDownloads.AmeriaBusiness.OutputFolder = cfg.OutputFolder
	dh.Config.BankDownloads.AmeriaBusiness.Cookie = ""
}

func (dh *DataHandler) recordMyAmeriaDownload(err error) {
	dh.setDownloadMetadata(&dh.Config.BankDownloads.MyAmeria.LastDownloadAt,
		&dh.Config.BankDownloads.MyAmeria.LastDownloadStatus,
		&dh.Config.BankDownloads.MyAmeria.LastDownloadError,
		err)
}

func (dh *DataHandler) recordAmeriaBusinessDownload(err error) {
	dh.setDownloadMetadata(&dh.Config.BankDownloads.AmeriaBusiness.LastDownloadAt,
		&dh.Config.BankDownloads.AmeriaBusiness.LastDownloadStatus,
		&dh.Config.BankDownloads.AmeriaBusiness.LastDownloadError,
		err)
}

func (dh *DataHandler) setDownloadMetadata(lastAt, lastStatus, lastError *string, err error) {
	*lastAt = time.Now().UTC().Format(time.RFC3339)
	if err != nil {
		*lastStatus = "error"
		*lastError = err.Error()
		return
	}
	*lastStatus = "ok"
	*lastError = ""
}

// GetSourceHealth returns per-source freshness and download state for the Files page.
func (dh *DataHandler) GetSourceHealth() []bankdownload.SourceHealth {
	return bankdownload.ComputeSourceHealth(dh.FileInfos, dh.Config)
}
