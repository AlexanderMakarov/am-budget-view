package bankdownload

import (
	"strings"

	"github.com/AlexanderMakarov/am-budget-view/internal/config"
	"github.com/AlexanderMakarov/am-budget-view/internal/model"
)

type SourceID string

const (
	SourceInecobankXML    SourceID = "inecobank_xml"
	SourceInecobankXLSX   SourceID = "inecobank_xlsx"
	SourceAmeriaBusiness  SourceID = "ameria_business"
	SourceMyAmeriaHistory SourceID = "my_ameria_history"
	SourceMyAmeriaGeneric SourceID = "my_ameria_generic"
	SourceArdshin         SourceID = "ardshin"
	SourceAcbaAccount     SourceID = "acba_account"
	SourceAcbaCard        SourceID = "acba_card"
)

// SourceDefinition describes a bank statement source and how to match files to it.
type SourceDefinition struct {
	ID                    SourceID
	Name                  string
	DocSourceID           string
	TagPrefixes           []string
	Glob                  func(*config.Config) string
	SupportsInAppDownload bool
	SupportsCliDownload   bool
}

// AllSources is the canonical registry in display order.
var AllSources = []SourceDefinition{
	{
		ID:          SourceInecobankXML,
		Name:        "Inecobank XML statement",
		DocSourceID: "inecobank",
		TagPrefixes: []string{"InecoXml:"},
		Glob: func(cfg *config.Config) string {
			return cfg.InecobankStatementXmlFilesGlob
		},
	},
	{
		ID:          SourceInecobankXLSX,
		Name:        "Inecobank XLSX statement",
		DocSourceID: "inecobank",
		TagPrefixes: []string{"InecoExcelRegular:", "InecoExcelCard:"},
		Glob: func(cfg *config.Config) string {
			return cfg.InecobankStatementXlsxFilesGlob
		},
	},
	{
		ID:                    SourceAmeriaBusiness,
		Name:                  "AmeriaBank CSV statement",
		DocSourceID:           "ameria-business",
		TagPrefixes:           []string{"AmeriaCsv:"},
		SupportsInAppDownload: true,
		SupportsCliDownload:   true,
		Glob: func(cfg *config.Config) string {
			return cfg.AmeriaCsvFilesGlob
		},
	},
	{
		ID:          SourceMyAmeriaHistory,
		Name:        "MyAmeria XLS / History",
		DocSourceID: "myameria",
		TagPrefixes: []string{"MyAmeriaXls:"},
		Glob: func(cfg *config.Config) string {
			history := cfg.MyAmeriaHistoryXlsFilesGlob
			statement := cfg.MyAmeriaAccountStatementXlsFilesGlob
			if history == "" {
				return statement
			}
			if statement == "" {
				return history
			}
			return history + ", " + statement
		},
	},
	{
		ID:                    SourceMyAmeriaGeneric,
		Name:                  "MyAmeria generic CSV",
		DocSourceID:           "myameria",
		TagPrefixes:           []string{"GenericCsv:"},
		SupportsInAppDownload: true,
		SupportsCliDownload:   true,
		Glob: func(cfg *config.Config) string {
			return cfg.GenericCsvFilesGlob
		},
	},
	{
		ID:          SourceArdshin,
		Name:        "Ardshinbank XLSX statement",
		DocSourceID: "ardshinbank",
		TagPrefixes: []string{"ArdshinXlsx:"},
		Glob: func(cfg *config.Config) string {
			return cfg.ArdshinbankXlsxFilesGlob
		},
	},
	{
		ID:          SourceAcbaAccount,
		Name:        "Acba Regular Account XLS statement",
		DocSourceID: "acba",
		TagPrefixes: []string{"AcbaAccountExcel:"},
		Glob: func(cfg *config.Config) string {
			return cfg.AcbaRegularAccountXlsFilesGlob
		},
	},
	{
		ID:          SourceAcbaCard,
		Name:        "Acba Card XLS statement",
		DocSourceID: "acba",
		TagPrefixes: []string{"AcbaCardExcel:"},
		Glob: func(cfg *config.Config) string {
			return cfg.AcbaCardXlsFilesGlob
		},
	},
}

var sourcesByID map[SourceID]SourceDefinition

func init() {
	sourcesByID = make(map[SourceID]SourceDefinition, len(AllSources))
	for _, def := range AllSources {
		sourcesByID[def.ID] = def
	}
}

// SourceByID returns the registry entry for id, or false if unknown.
func SourceByID(id SourceID) (SourceDefinition, bool) {
	def, ok := sourcesByID[id]
	return def, ok
}

// MatchFileInfo returns the SourceID for a parsed file, or empty if unmatched.
func MatchFileInfo(fi model.FileInfo) SourceID {
	if fi.Source == nil || fi.Source.Tag == "" {
		return ""
	}
	tag := fi.Source.Tag
	for _, def := range AllSources {
		for _, prefix := range def.TagPrefixes {
			if strings.HasPrefix(tag, prefix) {
				return def.ID
			}
		}
	}
	return ""
}
