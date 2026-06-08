package docs

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/yuin/goldmark"
)

var (
	embeddedFS embed.FS
	devMode    bool
)

// ValidSourceIDs lists bank documentation source identifiers.
var ValidSourceIDs = map[string]bool{
	"overview":        true,
	"inecobank":       true,
	"ameria-business": true,
	"myameria":        true,
	"ardshinbank":     true,
	"acba":            true,
	"generic":         true,
}

// Init configures the docs package with embedded filesystem and dev mode flag.
// When isDevMode is true, docs are read from the filesystem (docs/{lang}/banks/).
func Init(docsFS embed.FS, isDevMode bool) {
	embeddedFS = docsFS
	devMode = isDevMode
}

// NormalizeLang maps locale codes (en-US, ru-RU) to docs language folder (en, ru).
func NormalizeLang(lang string) string {
	lang = strings.ToLower(lang)
	if strings.HasPrefix(lang, "ru") {
		return "ru"
	}
	return "en"
}

// LoadBankDoc loads markdown bank documentation for the given language and source ID.
func LoadBankDoc(lang, sourceID string) (string, error) {
	if !ValidSourceIDs[sourceID] {
		return "", fmt.Errorf("unknown bank source ID: %s", sourceID)
	}

	shortLang := NormalizeLang(lang)
	path := fmt.Sprintf("docs/%s/banks/%s.md", shortLang, sourceID)

	var data []byte
	var err error
	if devMode {
		data, err = os.ReadFile(path)
	} else {
		data, err = fs.ReadFile(embeddedFS, path)
	}
	if err != nil {
		return "", fmt.Errorf("failed to load bank doc %s: %w", path, err)
	}
	return string(data), nil
}

// RenderHTML converts markdown to HTML using goldmark.
func RenderHTML(markdown string) (string, error) {
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(markdown), &buf); err != nil {
		return "", fmt.Errorf("failed to render markdown: %w", err)
	}
	return buf.String(), nil
}
