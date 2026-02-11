package currency

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexanderMakarov/am-budget-view/internal/i18n"
)

func TestMain(m *testing.M) {
	projectRoot := findProjectRoot()
	localesFS := os.DirFS(projectRoot)
	backend := i18n.I18nFsBackend{FS: localesFS}
	if err := i18n.Init(backend, "en-US", false); err != nil {
		log.Fatalf("Failed to initialize i18n for tests: %v", err)
	}
	os.Exit(m.Run())
}

func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			log.Fatal("Could not find project root (go.mod)")
		}
		dir = parent
	}
}
