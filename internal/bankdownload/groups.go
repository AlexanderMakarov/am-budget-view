package bankdownload

import (
	"sort"
	"time"

	"github.com/AlexanderMakarov/am-budget-view/internal/config"
	"github.com/AlexanderMakarov/am-budget-view/internal/model"
)

// FileSourceGroup is one bank/source with its parsed files and health summary.
type FileSourceGroup struct {
	Health        SourceHealth
	Files         []model.FileInfo
	FreshnessDays int // days since the newest file's modification time; -1 if no files
	IsStale       bool
	HasFiles      bool
}

// FileGroupsView lists every source in display order for the Files page.
type FileGroupsView struct {
	Groups []FileSourceGroup
}

// BuildFileGroups groups parsed files by source for the Files page table.
func BuildFileGroups(fileInfos []model.FileInfo, cfg *config.Config) FileGroupsView {
	return buildFileGroupsAt(fileInfos, cfg, time.Now())
}

func buildFileGroupsAt(fileInfos []model.FileInfo, cfg *config.Config, now time.Time) FileGroupsView {
	bySource := groupFileInfosBySource(fileInfos)
	allHealth := computeSourceHealthAt(fileInfos, cfg, now)
	healthByID := make(map[SourceID]SourceHealth, len(allHealth))
	for _, h := range allHealth {
		healthByID[h.SourceID] = h
	}

	staleThresholdDays := cfg.BankDownloads.StaleThresholdDays
	if staleThresholdDays == 0 {
		staleThresholdDays = 7
	}

	view := FileGroupsView{}
	for _, def := range AllSources {
		files := bySource[def.ID]
		h := healthByID[def.ID]
		group := FileSourceGroup{
			Health:   h,
			Files:    files,
			HasFiles: len(files) > 0,
		}
		if len(files) > 0 {
			sortFilesByModifiedDesc(files)
			group.FreshnessDays = freshnessDaysFromFiles(files, now)
			group.IsStale = group.FreshnessDays > staleThresholdDays
		} else {
			group.FreshnessDays = -1
		}
		view.Groups = append(view.Groups, group)
	}
	return view
}

func sortFilesByModifiedDesc(files []model.FileInfo) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModifiedTime.After(files[j].ModifiedTime)
	})
}

func freshnessDaysFromFiles(files []model.FileInfo, now time.Time) int {
	var latest time.Time
	for _, fi := range files {
		if fi.ModifiedTime.After(latest) {
			latest = fi.ModifiedTime
		}
	}
	if latest.IsZero() {
		return -1
	}
	return daysBetween(latest, now)
}
