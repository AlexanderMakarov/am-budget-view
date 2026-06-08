package config

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

const sinceDateLayout = "02-01-2006"

type BankDownloads struct {
	StaleThresholdDays int                          `yaml:"staleThresholdDays,omitempty"`
	MyAmeria           MyAmeriaDownloadConfig       `yaml:"myAmeria,omitempty"`
	AmeriaBusiness     AmeriaBusinessDownloadConfig `yaml:"ameriaBusiness,omitempty"`
}

type MyAmeriaDownloadConfig struct {
	Enabled            bool   `yaml:"enabled,omitempty"`
	ClientId           string `yaml:"clientId,omitempty"`
	AuthToken          string `yaml:"authToken,omitempty"`
	SinceDate          string `yaml:"sinceDate,omitempty"`
	LastDownloadAt     string `yaml:"lastDownloadAt,omitempty"`
	LastDownloadStatus string `yaml:"lastDownloadStatus,omitempty" validate:"omitempty,oneof=never ok error"`
	LastDownloadError  string `yaml:"lastDownloadError,omitempty"`
}

type AmeriaBusinessDownloadConfig struct {
	Enabled            bool   `yaml:"enabled,omitempty"`
	Cookie             string `yaml:"cookie,omitempty"`
	SinceDate          string `yaml:"sinceDate,omitempty"`
	OutputFolder       string `yaml:"outputFolder,omitempty"`
	LastDownloadAt     string `yaml:"lastDownloadAt,omitempty"`
	LastDownloadStatus string `yaml:"lastDownloadStatus,omitempty" validate:"omitempty,oneof=never ok error"`
	LastDownloadError  string `yaml:"lastDownloadError,omitempty"`
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
	Language                             string                        `yaml:"language,omitempty" validate:"omitempty,oneof=en ru"`
	EnsureTerminal                       bool                          `yaml:"ensureTerminal,omitempty"`
	UIPort                               int                           `yaml:"uiPort,omitempty"`
	InecobankStatementXmlFilesGlob       string                        `yaml:"inecobankStatementXmlFilesGlob" validate:"omitempty,filepath,min=1"`
	InecobankStatementXlsxFilesGlob      string                        `yaml:"inecobankStatementXlsxFilesGlob" validate:"omitempty,filepath,min=1"`
	AmeriaCsvFilesGlob                   string                        `yaml:"ameriaCsvFilesGlob" validate:"omitempty,filepath,min=1"`
	MyAmeriaAccountStatementXlsFilesGlob string                        `yaml:"myAmeriaAccountStatementXlsxFilesGlob" validate:"omitempty,filepath,min=1"`
	MyAmeriaHistoryXlsFilesGlob          string                        `yaml:"myAmeriaHistoryXlsFilesGlob" validate:"omitempty,filepath,min=1"`
	ArdshinbankXlsxFilesGlob             string                        `yaml:"ardshinbankXlsxFilesGlob,omitempty" validate:"omitempty,filepath,min=1"`
	AcbaRegularAccountXlsFilesGlob       string                        `yaml:"acbaRegularAccountXlsFilesGlob,omitempty" validate:"omitempty,filepath,min=1"`
	AcbaCardXlsFilesGlob                 string                        `yaml:"acbaCardXlsFilesGlob,omitempty" validate:"omitempty,filepath,min=1"`
	GenericCsvFilesGlob                  string                        `yaml:"genericCsvFilesGlob,omitempty" validate:"omitempty,filepath,min=1"`
	MyAmeriaMyAccounts                   map[string]string             `yaml:"myAmeriaMyAccounts,omitempty"`
	MyAccounts                           []string                      `yaml:"myAccounts,omitempty"`
	ExchangeRates                        map[string]map[string]float64 `yaml:"exchangeRates,omitempty"`
	ConvertToCurrencies                  []string                      `yaml:"convertToCurrencies,omitempty"`
	MinCurrencyTimespanPercent           int                           `yaml:"minCurrencyTimespanPercent,omitempty" validate:"min=0,max=100"`
	MaxCurrencyTimespanGapDays           int                           `yaml:"maxCurrencyTimespanGapDays,omitempty" validate:"min=0"`

	DetailedOutput             bool   `yaml:"detailedOutput"`
	CategorizeMode             bool   `yaml:"categorizeMode"`
	MonthStartDayNumber        uint   `yaml:"monthStartDayNumber,omitempty" validate:"min=1,max=31"`
	TimeZoneLocation           string `yaml:"timeZoneLocation,omitempty"`
	GroupAllUnknownTransactions bool   `yaml:"groupAllUnknownTransactions"`
	// Transactions categorization groups.
	Groups map[string]*GroupConfig `yaml:"groups,omitempty"`
	BankDownloads                  BankDownloads `yaml:"bankDownloads,omitempty"`
}

func ReadConfig(filename string) (*Config, error) {
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
	if cfg.UIPort == 0 {
		cfg.UIPort = 8080
	}
	if len(cfg.TimeZoneLocation) == 0 {
		tzname, err := tzlocal.RuntimeTZ()
		if err != nil {
			cfg.TimeZoneLocation = "UTC"
		} else {
			cfg.TimeZoneLocation = tzname
		}
	}
	if cfg.MinCurrencyTimespanPercent == 0 {
		cfg.MinCurrencyTimespanPercent = 80
	}
	if cfg.MaxCurrencyTimespanGapDays == 0 {
		cfg.MaxCurrencyTimespanGapDays = 30
	}
	if cfg.BankDownloads.StaleThresholdDays == 0 {
		cfg.BankDownloads.StaleThresholdDays = 7
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

	if err = validateBankDownloads(&cfg.BankDownloads); err != nil {
		return nil, err
	}

	// Validate other fields
	if err = validate.Struct(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validateBankDownloads(bd *BankDownloads) error {
	if err := validateMyAmeriaDownload(&bd.MyAmeria); err != nil {
		return err
	}
	return validateAmeriaBusinessDownload(&bd.AmeriaBusiness)
}

// ValidateBankDownloads checks bankDownloads fields the same way ReadConfig does.
func ValidateBankDownloads(bd *BankDownloads) error {
	if err := validateBankDownloads(bd); err != nil {
		return err
	}
	return validate.Struct(bd)
}

// MergeBankDownloads applies non-empty patch fields onto dst.
func MergeBankDownloads(dst *BankDownloads, patch BankDownloads) {
	if patch.StaleThresholdDays != 0 {
		dst.StaleThresholdDays = patch.StaleThresholdDays
	}
	mergeMyAmeriaDownloadConfig(&dst.MyAmeria, patch.MyAmeria)
	mergeAmeriaBusinessDownloadConfig(&dst.AmeriaBusiness, patch.AmeriaBusiness)
}

func mergeMyAmeriaDownloadConfig(dst *MyAmeriaDownloadConfig, patch MyAmeriaDownloadConfig) {
	if patch.ClientId != "" {
		dst.ClientId = patch.ClientId
	}
	if patch.AuthToken != "" {
		dst.AuthToken = patch.AuthToken
	}
	if patch.SinceDate != "" {
		dst.SinceDate = patch.SinceDate
	}
	if patch.LastDownloadAt != "" {
		dst.LastDownloadAt = patch.LastDownloadAt
	}
	if patch.LastDownloadStatus != "" {
		dst.LastDownloadStatus = patch.LastDownloadStatus
	}
	if patch.LastDownloadError != "" {
		dst.LastDownloadError = patch.LastDownloadError
	}
}

func mergeAmeriaBusinessDownloadConfig(dst *AmeriaBusinessDownloadConfig, patch AmeriaBusinessDownloadConfig) {
	if patch.Cookie != "" {
		dst.Cookie = patch.Cookie
	}
	if patch.SinceDate != "" {
		dst.SinceDate = patch.SinceDate
	}
	if patch.OutputFolder != "" {
		dst.OutputFolder = patch.OutputFolder
	}
	if patch.LastDownloadAt != "" {
		dst.LastDownloadAt = patch.LastDownloadAt
	}
	if patch.LastDownloadStatus != "" {
		dst.LastDownloadStatus = patch.LastDownloadStatus
	}
	if patch.LastDownloadError != "" {
		dst.LastDownloadError = patch.LastDownloadError
	}
}

func validateSinceDate(fieldName, value string) error {
	if value == "" {
		return fmt.Errorf("bankDownloads.%s is required", fieldName)
	}
	if _, err := time.Parse(sinceDateLayout, value); err != nil {
		return fmt.Errorf(
			"bankDownloads.%s must be in DD-MM-YYYY format, got %q",
			fieldName,
			value,
		)
	}
	return nil
}

// StripBankDownloadSecrets clears ephemeral session credentials from bankDownloads.
// Returns true if any secret was removed.
func StripBankDownloadSecrets(bd *BankDownloads) bool {
	changed := bd.MyAmeria.AuthToken != "" || bd.AmeriaBusiness.Cookie != ""
	bd.MyAmeria.AuthToken = ""
	bd.AmeriaBusiness.Cookie = ""
	return changed
}

func validateMyAmeriaDownload(cfg *MyAmeriaDownloadConfig) error {
	if cfg.SinceDate == "" {
		return nil
	}
	return validateSinceDate("myAmeria.sinceDate", cfg.SinceDate)
}

func validateAmeriaBusinessDownload(cfg *AmeriaBusinessDownloadConfig) error {
	if cfg.SinceDate == "" {
		return nil
	}
	return validateSinceDate("ameriaBusiness.sinceDate", cfg.SinceDate)
}

// WriteToFile writes the configuration to a file with preserving comments.
func (cfg *Config) WriteToFile(filename string) error {
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

// mergeComments recursively copies comments from the old node to the new node.
func mergeComments(newNode, oldNode *yaml.Node) {
	// Skip comment merging for the 'substrings' key node to prevent comment duplication.
	if newNode.Kind == yaml.ScalarNode && newNode.Value == "substrings" {
		return
	}

	// Copy comments from the old node to the new node.
	if oldNode.HeadComment != "" {
		newNode.HeadComment = oldNode.HeadComment
	}
	if oldNode.LineComment != "" {
		newNode.LineComment = oldNode.LineComment
	}
	if oldNode.FootComment != "" {
		newNode.FootComment = oldNode.FootComment
	}

	// Recursively merge comments for mapping nodes.
	if len(newNode.Content) > 0 && len(oldNode.Content) > 0 {
		if newNode.Kind == yaml.MappingNode && oldNode.Kind == yaml.MappingNode {
			oldKeyToIndex := make(map[string]int)
			for i := 0; i+1 < len(oldNode.Content); i += 2 {
				key := oldNode.Content[i]
				oldKeyToIndex[key.Value] = i
			}
			for i := 0; i+1 < len(newNode.Content); i += 2 {
				newKey := newNode.Content[i]
				if oi, ok := oldKeyToIndex[newKey.Value]; ok {
					oldKey := oldNode.Content[oi]
					oldVal := oldNode.Content[oi+1]
					mergeComments(newKey, oldKey)
					mergeComments(newNode.Content[i+1], oldVal)
				}
			}
		} else if newNode.Kind == yaml.SequenceNode && oldNode.Kind == yaml.SequenceNode {
			for i := 0; i < len(newNode.Content) && i < len(oldNode.Content); i++ {
				mergeComments(newNode.Content[i], oldNode.Content[i])
			}
		} else {
			for i := 0; i < len(newNode.Content) && i < len(oldNode.Content); i++ {
				mergeComments(newNode.Content[i], oldNode.Content[i])
			}
		}
	}
}
