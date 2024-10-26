package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/thlib/go-timezone-local/tzlocal"
	"gopkg.in/yaml.v3"
)

type Config struct {
	InecobankStatementXmlFilesGlob       string              `yaml:"inecobankStatementXmlFilesGlob" validate:"required,filepath,min=1"`
	InecobankStatementXlsxFilesGlob      string              `yaml:"inecobankStatementXlsxFilesGlob" validate:"required,filepath,min=1"`
	AmeriaCsvFilesGlob                   string              `yaml:"ameriaCsvFilesGlob" validate:"required,filepath,min=1"`
	MyAmeriaAccountStatementXlsFilesGlob string              `yaml:"myAmeriaAccountStatementXlsxFilesGlob" validate:"required,filepath,min=1"`
	MyAmeriaHistoryXlsFilesGlob          string              `yaml:"myAmeriaHistoryXlsFilesGlob" validate:"required,filepath,min=1"`
	MyAmeriaMyAccounts                   []string            `yaml:"myAmeriaMyAccounts,omitempty"`
	ConvertToCurrencies                  []string            `yaml:"convertToCurrencies,omitempty"`
	MinCurrencyTimespanPercent           int                 `yaml:"minCurrencyTimespanPercent,omitempty" validate:"min=0,max=100"`
	MaxCurrencyTimespanGapsDays          int                 `yaml:"maxCurrencyTimespanGapsDays,omitempty" validate:"min=0"`
	DetailedOutput                       bool                `yaml:"detailedOutput"`
	CategorizeMode                       bool                `yaml:"categorizeMode"`
	MonthStartDayNumber                  uint                `yaml:"monthStartDayNumber,omitempty" validate:"min=1,max=31" default:"1"`
	TimeZoneLocation                     string              `yaml:"timeZoneLocation,omitempty" validate:"timezone"`
	GroupAllUnknownTransactions          bool                `yaml:"groupAllUnknownTransactions"`
	IgnoreSubstrings                     []string            `yaml:"ignoreSubstrings,omitempty"`
	GroupNamesToSubstrings               map[string][]string `yaml:"groupNamesToSubstrings"`
}

func readConfig(filename string) (*Config, error) {
	buf, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	decoder := yaml.NewDecoder(strings.NewReader(string(buf)))
	decoder.KnownFields(true) // Disallow unknown fields
	if err = decoder.Decode(cfg); err != nil {
		if err.Error() == "EOF" {
			return nil, fmt.Errorf("can't decode YAML from configuration file '%s': %v", filename, err)
		}
		return nil, err
	}

	// Set default values.
	if cfg.MonthStartDayNumber == 0 {
		cfg.MonthStartDayNumber = 1
	}
	if len(cfg.TimeZoneLocation) == 0 {
		tzname, err := tzlocal.RuntimeTZ()
		if err != nil {
			return nil, err
		}
		cfg.TimeZoneLocation = tzname
	}

	// Validate.
	validate := validator.New()
	if err = validate.Struct(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
