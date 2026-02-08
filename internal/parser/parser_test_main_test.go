package parser

import (
	"io/fs"
	"log"
	"os"
	"testing"

	"github.com/AlexanderMakarov/am-budget-view/internal/i18n"
)

func TestMain(m *testing.M) {
	// Find project root by looking for go.mod
	projectRoot := findProjectRoot()
	localesFS := os.DirFS(projectRoot)
	backend := i18n.I18nFsBackend{FS: localesFS.(fs.ReadFileFS)}
	if err := i18n.Init(backend, "en-US", false); err != nil {
		log.Fatalf("Failed to initialize i18n for tests: %v", err)
	}
	os.Exit(m.Run())
}

func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir
		}
		parent := dir[:max(0, len(dir)-1)]
		for parent != "" && parent[len(parent)-1] != '/' {
			parent = parent[:len(parent)-1]
		}
		if parent == "" || parent == dir {
			log.Fatal("Could not find project root (go.mod)")
		}
		dir = parent[:len(parent)-1]
	}
}
