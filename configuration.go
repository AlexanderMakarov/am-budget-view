package main

import (
	"fmt"
	"os"
	"time"

	_ "time/tzdata"

	"github.com/go-playground/validator/v10"
	"github.com/thlib/go-timezone-local/tzlocal"
	"gopkg.in/yaml.v3"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	_ = validate.RegisterValidation("timezone", validateTimezone)
}

func validateTimezone(fl validator.FieldLevel) bool {
	timezone := fl.Field().String()
	if timezone == "" {
		return true // Empty timezone is allowed, will be replaced with system default
	}
	_, err := time.LoadLocation(timezone)
	return err == nil
}

type GroupConfig struct {
	// Substrings to match in transaction description.
	Substrings []string `yaml:"substrings,omitempty"`
	// Accounts to match in "payee" field.
	FromAccounts []string `yaml:"fromAccounts,omitempty"`
	// Accounts to match in "receiver" field.
	ToAccounts []string `yaml:"toAccounts,omitempty"`
}

// Config represents the application configuration.
type Config struct {
	Language                             string            `yaml:"language,omitempty" validate:"omitempty,oneof=en ru"`
	EnsureTerminal                       bool              `yaml:"ensureTerminal,omitempty" default:"true"`
	UIPort                               int               `yaml:"uiPort,omitempty" default:"8080"`
	InecobankStatementXmlFilesGlob       string            `yaml:"inecobankStatementXmlFilesGlob" validate:"omitempty,filepath,min=1"`
	InecobankStatementXlsxFilesGlob      string            `yaml:"inecobankStatementXlsxFilesGlob" validate:"omitempty,filepath,min=1"`
	AmeriaCsvFilesGlob                   string            `yaml:"ameriaCsvFilesGlob" validate:"omitempty,filepath,min=1"`
	MyAmeriaAccountStatementXlsFilesGlob string            `yaml:"myAmeriaAccountStatementXlsxFilesGlob" validate:"omitempty,filepath,min=1"`
	MyAmeriaHistoryXlsFilesGlob          string            `yaml:"myAmeriaHistoryXlsFilesGlob" validate:"omitempty,filepath,min=1"`
	ArdshinbankXlsxFilesGlob             string            `yaml:"ardshinbankXlsxFilesGlob,omitempty" validate:"omitempty,filepath,min=1"`
	AcbaRegularAccountXlsFilesGlob       string            `yaml:"acbaRegularAccountXlsFilesGlob,omitempty" validate:"omitempty,filepath,min=1"`
	AcbaCardXlsFilesGlob                 string            `yaml:"acbaCardXlsFilesGlob,omitempty" validate:"omitempty,filepath,min=1"`
	GenericCsvFilesGlob                  string            `yaml:"genericCsvFilesGlob,omitempty" validate:"omitempty,filepath,min=1"`
	MyAmeriaMyAccounts                   map[string]string `yaml:"myAmeriaMyAccounts,omitempty"`
	MyAccounts                           []string          `yaml:"myAccounts,omitempty"`
	ConvertToCurrencies                  []string          `yaml:"convertToCurrencies,omitempty"`
	MinCurrencyTimespanPercent           int               `yaml:"minCurrencyTimespanPercent,omitempty" validate:"min=0,max=100"`
	MaxCurrencyTimespanGapsDays          int               `yaml:"maxCurrencyTimespanGapsDays,omitempty" validate:"min=0"`

	DetailedOutput                       bool              `yaml:"detailedOutput"`
	CategorizeMode                       bool              `yaml:"categorizeMode"`
	MonthStartDayNumber                  uint              `yaml:"monthStartDayNumber,omitempty" validate:"min=1,max=31" default:"1"`
	TimeZoneLocation                     string            `yaml:"timeZoneLocation,omitempty"`
	GroupAllUnknownTransactions          bool              `yaml:"groupAllUnknownTransactions"`
	// Transactions categorization groups.
	Groups map[string]*GroupConfig `yaml:"groups,omitempty"`
}

func readConfig(filename string) (*Config, error) {
	buf, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// First unmarshal into a Node to preserve structure.
	var node yaml.Node
	if err := yaml.Unmarshal(buf, &node); err != nil {
		if err.Error() == "EOF" {
			return nil, fmt.Errorf("can't decode YAML from configuration file '%s': %v", filename, err)
		}
		return nil, err
	}

	// Then decode into the config struct.
	cfg := &Config{}
	if err := node.Decode(cfg); err != nil {
		return nil, err
	}

	// Set default values.
	if cfg.MonthStartDayNumber == 0 {
		cfg.MonthStartDayNumber = 1
	}
	if len(cfg.TimeZoneLocation) == 0 {
		tzname, err := tzlocal.RuntimeTZ()
		if err != nil {
			// Fallback to UTC if system timezone cannot be determined
			cfg.TimeZoneLocation = "UTC"
		} else {
			cfg.TimeZoneLocation = tzname
		}
	}

	// Verify timezone is valid
	_, err = time.LoadLocation(cfg.TimeZoneLocation)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone location '%s': %w", cfg.TimeZoneLocation, err)
	}

	// Check that Groups is set
	if len(cfg.Groups) == 0 {
		return nil, fmt.Errorf("'groups' must be set")
	}

	// Validate other fields
	if err = validate.Struct(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// writeToFile writes the configuration to a file with preserving comments.
// Note that comments are preserved with following limitations:
// - Optional fields will be added with default values.
// - Order of fields is replaced with `Config` struct fields order.
// - Single quotes are changed to double quotes.
// - 2 spaces before comments are changed to 1.
func (cfg *Config) writeToFile(filename string) error {
	// First read the existing file to get the node with comments
	var oldNode yaml.Node
	if existingContent, err := os.ReadFile(filename); err == nil {
		if err := yaml.Unmarshal(existingContent, &oldNode); err != nil {
			return err
		}
	}

	// Create a new node from the current config
	var newNode yaml.Node
	buf, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(buf, &newNode); err != nil {
		return err
	}

	// If we have an existing node, merge the comments
	if oldNode.Content != nil {
		mergeComments(&newNode, &oldNode)
	}

	// Write the result back to file.
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := yaml.NewEncoder(f)
	encoder.SetIndent(2)
	return encoder.Encode(&newNode)
}

// mergeComments recursively copies comments from the old node to the new node
func mergeComments(newNode, oldNode *yaml.Node) {
	// Skip comment merging for the 'substrings' key node to prevent comment duplication
	if newNode.Kind == yaml.ScalarNode && newNode.Value == "substrings" {
		return
	}

	if oldNode.HeadComment != "" {
		newNode.HeadComment = oldNode.HeadComment
	}
	if oldNode.LineComment != "" {
		newNode.LineComment = oldNode.LineComment
	}
	if oldNode.FootComment != "" {
		newNode.FootComment = oldNode.FootComment
	}

	// Recursively merge comments for mapping nodes
	if len(newNode.Content) > 0 && len(oldNode.Content) > 0 {
		// Continue with regular comment merging for other nodes
		for i := 0; i < len(newNode.Content) && i < len(oldNode.Content); i++ {
			mergeComments(newNode.Content[i], oldNode.Content[i])
		}
	}
}
