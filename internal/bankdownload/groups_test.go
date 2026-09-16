package bankdownload

import (
	"testing"
	"time"

	"github.com/AlexanderMakarov/am-budget-view/internal/config"
	"github.com/AlexanderMakarov/am-budget-view/internal/model"
)

func TestBuildFileGroups_SortsByModifiedDesc(t *testing.T) {
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	old := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)

	olderFile := fileInfo("InecoXml:AMD", old, old)
	olderFile.Path = "older.xml"
	olderFile.ModifiedTime = old

	newerFile := fileInfo("InecoXml:AMD", newer, newer)
	newerFile.Path = "newer.xml"
	newerFile.ModifiedTime = newer

	cfg := &config.Config{BankDownloads: config.BankDownloads{StaleThresholdDays: 7}}
	groups := buildFileGroupsAt([]model.FileInfo{olderFile, newerFile}, cfg, now)
	var inecoGroup *FileSourceGroup
	for i := range groups.Groups {
		if groups.Groups[i].Health.SourceID == SourceInecobankXML {
			inecoGroup = &groups.Groups[i]
			break
		}
	}
	if inecoGroup == nil || !inecoGroup.HasFiles {
		t.Fatal("expected inecobank_xml group with files")
	}
	files := inecoGroup.Files
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2", len(files))
	}
	if files[0].Path != "newer.xml" {
		t.Errorf("first file = %q, want newer.xml", files[0].Path)
	}
}

func TestBuildFileGroups_FreshnessDays(t *testing.T) {
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	modified := time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)

	fi := fileInfo("InecoXml:AMD", modified, modified)
	fi.ModifiedTime = modified

	cfg := &config.Config{BankDownloads: config.BankDownloads{StaleThresholdDays: 7}}
	groups := buildFileGroupsAt([]model.FileInfo{fi}, cfg, now)
	var inecoGroup *FileSourceGroup
	for i := range groups.Groups {
		if groups.Groups[i].Health.SourceID == SourceInecobankXML {
			inecoGroup = &groups.Groups[i]
			break
		}
	}
	if inecoGroup == nil {
		t.Fatal("expected inecobank_xml group")
	}
	if inecoGroup.FreshnessDays != 5 {
		t.Errorf("FreshnessDays = %d, want 5", inecoGroup.FreshnessDays)
	}
	if inecoGroup.IsStale {
		t.Error("expected IsStale false for 5-day-old file with 7-day threshold")
	}

	staleCfg := &config.Config{BankDownloads: config.BankDownloads{StaleThresholdDays: 3}}
	staleGroups := buildFileGroupsAt([]model.FileInfo{fi}, staleCfg, now)
	for i := range staleGroups.Groups {
		if staleGroups.Groups[i].Health.SourceID == SourceInecobankXML {
			if !staleGroups.Groups[i].IsStale {
				t.Error("expected IsStale true for 5-day-old file with 3-day threshold")
			}
			break
		}
	}
}
