package docs

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "docs", "en", "banks", "overview.md")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root with docs/en/banks/overview.md")
		}
		dir = parent
	}
}

func TestNormalizeLang(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"en", "en"},
		{"en-US", "en"},
		{"ru", "ru"},
		{"ru-RU", "ru"},
		{"", "en"},
	}
	for _, tt := range tests {
		if got := NormalizeLang(tt.input); got != tt.expected {
			t.Errorf("NormalizeLang(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestLoadBankDocAllSourcesDevMode(t *testing.T) {
	root := findRepoRoot(t)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	Init(embed.FS{}, true)

	for sourceID := range ValidSourceIDs {
		for _, lang := range []string{"en", "en-US", "ru", "ru-RU"} {
			content, err := LoadBankDoc(lang, sourceID)
			if err != nil {
				t.Errorf("LoadBankDoc(%q, %q) error: %v", lang, sourceID, err)
				continue
			}
			if len(content) == 0 {
				t.Errorf("LoadBankDoc(%q, %q) returned empty content", lang, sourceID)
			}
		}
	}
}

func TestLoadBankDocUnknownSource(t *testing.T) {
	Init(embed.FS{}, false)

	_, err := LoadBankDoc("en", "unknown-bank")
	if err == nil {
		t.Fatal("expected error for unknown source ID")
	}
}

func TestBankDocsKeepEnglishAndRussianStructureInSync(t *testing.T) {
	root := findRepoRoot(t)
	for sourceID := range ValidSourceIDs {
		en, err := os.ReadFile(filepath.Join(root, "docs", "en", "banks", sourceID+".md"))
		if err != nil {
			t.Fatal(err)
		}
		ru, err := os.ReadFile(filepath.Join(root, "docs", "ru", "banks", sourceID+".md"))
		if err != nil {
			t.Fatal(err)
		}
		enStructure := markdownStructure(string(en))
		ruStructure := markdownStructure(string(ru))
		if strings.Join(enStructure, "\n") != strings.Join(ruStructure, "\n") {
			t.Errorf("docs for %s have different EN/RU structure:\nEN %v\nRU %v", sourceID, enStructure, ruStructure)
		}
	}
}

func markdownStructure(markdown string) []string {
	var result []string
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			continue
		case strings.HasPrefix(line, "#"):
			result = append(result, fmt.Sprintf("heading:%d", len(line)-len(strings.TrimLeft(line, "#"))))
		case strings.HasPrefix(trimmed, "```"):
			result = append(result, "fence")
		case strings.HasPrefix(trimmed, "- "):
			result = append(result, fmt.Sprintf("bullet:%d", len(line)-len(strings.TrimLeft(line, " "))))
		case len(trimmed) >= 3 && trimmed[0] >= '0' && trimmed[0] <= '9' && trimmed[1:3] == ". ":
			result = append(result, "numbered")
		default:
			result = append(result, "paragraph")
		}
	}
	return result
}

func TestRenderHTML(t *testing.T) {
	html, err := RenderHTML("# Title\n\nSome **bold** text.")
	if err != nil {
		t.Fatalf("RenderHTML error: %v", err)
	}
	if !strings.Contains(html, "<h1>Title</h1>") {
		t.Errorf("expected h1 tag in output, got: %s", html)
	}
	if !strings.Contains(html, "<strong>bold</strong>") {
		t.Errorf("expected strong tag in output, got: %s", html)
	}
}
