package main

import (
	"errors"
	"os"
	"testing"

	"github.com/thlib/go-timezone-local/tzlocal"
)

func TestReadConfig_ValidYAML(t *testing.T) {
	// Arrange
	tempFile := createTempFileWithContent(
		`inecobankStatementXmlFilesGlob: "*.xml"
inecobankStatementXlsxFilesGlob: "*.xlsx"
ameriaCsvFilesGlob: "*.csv"
myAmeriaAccountStatementXlsxFilesGlob: "*.xls"
myAmeriaHistoryXlsFilesGlob: "History*.xls"
ardshinbankXlsxFilesGlob: "STATEMENT_*.xlsx"
genericCsvFilesGlob: "generic*.csv"
myAmeriaMyAccounts: 
  "Account1": "USD"
  "Account2": "AMD"
detailedOutput: true
categorizeMode: false
monthStartDayNumber: 1
timeZoneLocation: "America/New_York"
groupAllUnknownTransactions: true
groupNamesToSubstrings:
  g1:
    - Sub1
    - Sub2
  g2:
    - Sub3
`,
	)
	defer os.Remove(tempFile.Name())

	// Act
	cfg, err := readConfig(tempFile.Name())

	// Assert
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
	if cfg == nil {
		t.Error("Expected config, but got nil")
		return
	}
	if cfg.InecobankStatementXmlFilesGlob != "*.xml" {
		t.Errorf(
			"Expected InecobankStatementXmlFilesGlob to be '*.xml', got '%s'",
			cfg.InecobankStatementXmlFilesGlob,
		)
	}
	if cfg.AmeriaCsvFilesGlob != "*.csv" {
		t.Errorf(
			"Expected AmeriaCsvFilesGlob to be '*.csv', got '%s'",
			cfg.AmeriaCsvFilesGlob,
		)
	}
	if cfg.MyAmeriaAccountStatementXlsFilesGlob != "*.xls" {
		t.Errorf(
			"Expected MyAmeriaAccountStatementXlsFilesGlob to be '*.xls', got '%s'",
			cfg.MyAmeriaAccountStatementXlsFilesGlob,
		)
	}
	if cfg.MyAmeriaHistoryXlsFilesGlob != "History*.xls" {
		t.Errorf(
			"Expected MyAmeriaHistoryXlsFilesGlob to be 'History*.xls', got '%s'",
			cfg.MyAmeriaHistoryXlsFilesGlob,
		)
	}
	if cfg.ArdshinbankCsvFilesGlob != "STATEMENT_*.xlsx" {
		t.Errorf(
			"Expected ArdshinbankCsvFilesGlob to be 'STATEMENT_*.xlsx', got '%s'",
			cfg.ArdshinbankCsvFilesGlob,
		)
	}
	if cfg.GenericCsvFilesGlob != "generic*.csv" {
		t.Errorf(
			"Expected GenericCsvFilesGlob to be 'generic*.csv', got '%s'",
			cfg.GenericCsvFilesGlob,
		)
	}
	if len(cfg.MyAmeriaMyAccounts) != 2 || cfg.MyAmeriaMyAccounts["Account1"] != "USD" || cfg.MyAmeriaMyAccounts["Account2"] != "AMD" {
		t.Errorf(
			"Expected MyAmeriaMyAccounts to be {'Account1': 'USD', 'Account2': 'AMD'}, got '%v'",
			cfg.MyAmeriaMyAccounts,
		)
	}
	if !cfg.DetailedOutput {
		t.Error("Expected DetailedOutput to be true")
	}
	if cfg.CategorizeMode {
		t.Error("Expected CategorizeMode to be false")
	}
	if cfg.MonthStartDayNumber != 1 {
		t.Errorf("Expected MonthStartDayNumber to be 1, got '%d'", cfg.MonthStartDayNumber)
	}
	if cfg.TimeZoneLocation != "America/New_York" {
		t.Errorf("Expected TimeZoneLocation to be 'America/New_York', got '%s'", cfg.TimeZoneLocation)
	}
	if !cfg.GroupAllUnknownTransactions {
		t.Error("Expected GroupAllUnknownTransactions to be true")
	}
	if len(cfg.GroupNamesToSubstrings) != 2 || len(cfg.GroupNamesToSubstrings["g1"]) != 2 || cfg.GroupNamesToSubstrings["g1"][0] != "Sub1" || cfg.GroupNamesToSubstrings["g1"][1] != "Sub2" || len(cfg.GroupNamesToSubstrings["g2"]) != 1 || cfg.GroupNamesToSubstrings["g2"][0] != "Sub3" {
		t.Errorf(
			"Expected GroupNamesToSubstrings to have correct mappings, got '%v'",
			cfg.GroupNamesToSubstrings,
		)
	}
}

func TestReadConfig_InvalidYAML(t *testing.T) {
	// Arrange. Note that "myAmeriaMyAccounts" doesn't have ":" at the end.
	tempFile := createTempFileWithContent(
		`inecobankStatementXmlFilesGlob: "*.xml"
inecobankStatementXlsxFilesGlob: "*.xlsx"
ameriaCsvFilesGlob: "*.csv"
myAmeriaAccountStatementXlsxFilesGlob: "*.xls"
myAmeriaHistoryXlsFilesGlob: "History*.xls"
ardshinbankXlsxFilesGlob: "STATEMENT_*.xlsx"
genericCsvFilesGlob: "generic*.csv"
myAmeriaMyAccounts
  - Account1
  - Account2
detailedOutput: true
monthStartDayNumber: 1
timeZoneLocation: "America/New_York"
groupAllUnknownTransactions: true
groupNamesToSubstrings:
  g1:
    - Sub1
    - Sub2
  g2:
    - Sub3
`,
	)
	defer os.Remove(tempFile.Name())

	// Act
	_, err := readConfig(tempFile.Name())

	// Assert
	if err == nil {
		t.Fatal("Expected error, but got no error")
	}
	checkErrorContainsSubstring(t, err, "yaml: line 8: could not find expected ':'")
}

func TestReadConfig_GroupsNotSpecified(t *testing.T) {
	// Arrange. Note that both "groupNamesToSubstrings" and "groups" are not specified.
	tempFile := createTempFileWithContent(
		`inecobankStatementXmlFilesGlob: "*.xml"
inecobankStatementXlsxFilesGlob: "*.xlsx"
ameriaCsvFilesGlob: "*.csv"
myAmeriaAccountStatementXlsxFilesGlob: "*.xls"
myAmeriaHistoryXlsFilesGlob: "History*.xls"
ardshinbankXlsxFilesGlob: "STATEMENT_*.xlsx"
genericCsvFilesGlob: "generic*.csv"
myAmeriaMyAccounts: 
  Account1: USD
  Account2: AMD
detailedOutput: true
monthStartDayNumber: 1
timeZoneLocation: "America/New_York"
groupAllUnknownTransactions: true
groupsNamesToSubstrings: # Should be groupNamesToSubstrings
  g1:
    - Sub1
    - Sub2
  g2:
    - Sub3
`,
	)
	defer os.Remove(tempFile.Name())

	// Act
	_, err := readConfig(tempFile.Name())

	// Assert
	if err == nil {
		t.Fatal("Expected error, but got no error")
	}
	checkErrorContainsSubstring(t, err, "either 'groups' or 'groupNamesToSubstrings' must be set")
}

func TestReadConfig_MisstypedField(t *testing.T) {
	// Arrange.
	tempFile := createTempFileWithContent(
		`inecobankStatementXmlFilesGlob: "*.xml"
inecobankStatementXlsxFilesGlob: "*.xlsx"
myAmeriaCsvFilesGlob: "*.csv" # Should be ameriaCsvFilesGlob
myAmeriaAccountStatementXlsxFilesGlob: "*.xls"
myAmeriaHistoryXlsFilesGlob: "History*.xls"
ardshinbankXlsxFilesGlob: "STATEMENT_*.xlsx"
genericCsvFilesGlob: "generic*.csv"
myAmeriaMyAccounts: 
  Account1: USD
  Account2: AMD
detailedOutput: true
monthStartDayNumber: 1
timeZoneLocation: "America/New_York"
groupAllUnknownTransactions: true
groups:
  g1:
    substrings:
      - Sub1
      - Sub2
    fromAccounts:
      - "1234567890123456"
  g2:
    substrings:
      - Sub3
    fromAccounts:
      - "1234567890123456"
`,
	)
	defer os.Remove(tempFile.Name())

	// Act
	_, err := readConfig(tempFile.Name())

	// Assert
	if err == nil {
		t.Fatal("Expected error, but got no error")
	}
	checkErrorContainsSubstring(
		t,
		err,
		"Key: 'Config.AmeriaCsvFilesGlob' Error:Field validation for 'AmeriaCsvFilesGlob' failed on the 'required' tag",
	)
}

func TestReadConfig_FileNotFound(t *testing.T) {
	// Arrange
	nonexistentFile := "nonexistent_file.yaml"

	// Act
	_, err := readConfig(nonexistentFile)

	// Assert
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Expected os.ErrNotExist error, but got: %v", err)
	}
}

func TestReadConfig_EmptyFile(t *testing.T) {
	// Arrange
	tempFile := createTempFileWithContent("")
	defer os.Remove(tempFile.Name())

	// Act
	_, err := readConfig(tempFile.Name())

	// Assert
	if err == nil {
		t.Fatal("Expected error, but got no error")
	}
	checkErrorContainsSubstring(
		t,
		err,
		"either 'groups' or 'groupNamesToSubstrings' must be set",
	)
}

func TestReadConfig_NotAllFields(t *testing.T) {
	// Arrange
	tempFile := createTempFileWithContent(
		`inecobankStatementXmlFilesGlob: "*.xml"
inecobankStatementXlsxFilesGlob: "*.xlsx"
ameriaCsvFilesGlob: "*.csv"
myAmeriaAccountStatementXlsxFilesGlob: "*.xls"
myAmeriaHistoryXlsFilesGlob: "History*.xls"
genericCsvFilesGlob: "generic*.csv"
detailedOutput: false
groupAllUnknownTransactions: true
groupNamesToSubstrings:
  g1:
    - Sub1
    - Sub2
  g2:
    - Sub3
`,
	)
	defer os.Remove(tempFile.Name())

	// Act
	cfg, err := readConfig(tempFile.Name())

	// Assert
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
	if cfg == nil {
		t.Error("Expected config, but got nil")
		return
	}
	if cfg.InecobankStatementXmlFilesGlob != "*.xml" {
		t.Errorf(
			"Expected InecobankStatementXmlFilesGlob to be '*.xml', got '%s'",
			cfg.InecobankStatementXmlFilesGlob,
		)
	}
	if cfg.AmeriaCsvFilesGlob != "*.csv" {
		t.Errorf(
			"Expected AmeriaCsvFilesGlob to be '*.csv', got '%s'",
			cfg.AmeriaCsvFilesGlob,
		)
	}
	if cfg.MyAmeriaAccountStatementXlsFilesGlob != "*.xls" {
		t.Errorf(
			"Expected MyAmeriaAccountStatementXlsFilesGlob to be '*.xls', got '%s'",
			cfg.MyAmeriaAccountStatementXlsFilesGlob,
		)
	}
	if cfg.MyAmeriaHistoryXlsFilesGlob != "History*.xls" {
		t.Errorf(
			"Expected MyAmeriaHistoryXlsFilesGlob to be 'History*.xls', got '%s'",
			cfg.MyAmeriaHistoryXlsFilesGlob,
		)
	}
	if cfg.GenericCsvFilesGlob != "generic*.csv" {
		t.Errorf(
			"Expected GenericCsvFilesGlob to be 'generic*.csv', got '%s'",
			cfg.GenericCsvFilesGlob,
		)
	}
	if len(cfg.MyAmeriaMyAccounts) != 0 {
		t.Errorf(
			"Expected MyAmeriaMyAccounts to be empty, got '%v'",
			cfg.MyAmeriaMyAccounts,
		)
	}
	if cfg.DetailedOutput {
		t.Error("Expected DetailedOutput to be false")
	}
	if cfg.MonthStartDayNumber != 1 {
		t.Errorf("Expected MonthStartDayNumber to be 1, got '%d'", cfg.MonthStartDayNumber)
	}
	tzname, _ := tzlocal.RuntimeTZ()
	if cfg.TimeZoneLocation != tzname {
		t.Errorf("Expected TimeZoneLocation to be '%s', got '%s'", tzname, cfg.TimeZoneLocation)
	}
	if !cfg.GroupAllUnknownTransactions {
		t.Error("Expected GroupAllUnknownTransactions to be true")
	}
	if len(cfg.GroupNamesToSubstrings) != 2 || len(cfg.GroupNamesToSubstrings["g1"]) != 2 || cfg.GroupNamesToSubstrings["g1"][0] != "Sub1" || cfg.GroupNamesToSubstrings["g1"][1] != "Sub2" || len(cfg.GroupNamesToSubstrings["g2"]) != 1 || cfg.GroupNamesToSubstrings["g2"][0] != "Sub3" {
		t.Errorf(
			"Expected GroupNamesToSubstrings to have correct mappings, got '%v'",
			cfg.GroupNamesToSubstrings,
		)
	}
}

func readUseWriteConfig(t *testing.T, content string) string {
	tempFile := createTempFileWithContent(content)
	defer os.Remove(tempFile.Name())
	cfg, err := readConfig(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	_, err = NewCategorization(cfg)
	if err != nil {
		t.Fatalf("Failed to create categorization: %v", err)
	}

	err = cfg.writeToFile(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Read resulting file to check for comments.
	buf, err := os.ReadFile(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to read written config: %v", err)
	}
	return string(buf)
}

func TestWriteToFile_FromNewToNew(t *testing.T) {
	// Arrange.
	initialContent := `# Root comment
inecobankStatementXmlFilesGlob: '*.xml'  # After line comment
inecobankStatementXlsxFilesGlob: "*.xlsx"
ameriaCsvFilesGlob: '*.csv'
myAmeriaAccountStatementXlsxFilesGlob: '*.xls'
myAmeriaHistoryXlsFilesGlob: "1324657890123456"
ardshinbankXlsxFilesGlob: "STATEMENT_*.xlsx"
genericCsvFilesGlob: "generic*.csv"
# Before group comment
myAmeriaMyAccounts:
  Account1: USD  # List element comment
  # Between list elements comment
  Account2: AMD
detailedOutput: true
categorizeMode: false
monthStartDayNumber: 1
timeZoneLocation: America/New_York
groupAllUnknownTransactions: true
groups:
  # Before group comment
  g1:
    substrings:
      - Sub1 # Group element comment
      # Before group element comment
      - Sub2
`
	// Note that comments are preserved with following limitations:
	// - Optional fields will be added with default values.
	// - Order of fields is replaced with `Config` struct fields order.
	// - 2 spaces before comments are changed to 1.
	// - Single quotes are changed to double quotes.
	expectedContent := `# Root comment
inecobankStatementXmlFilesGlob: '*.xml' # After line comment
inecobankStatementXlsxFilesGlob: '*.xlsx'
ameriaCsvFilesGlob: '*.csv'
myAmeriaAccountStatementXlsxFilesGlob: '*.xls'
myAmeriaHistoryXlsFilesGlob: "1324657890123456"
ardshinbankXlsxFilesGlob: STATEMENT_*.xlsx
genericCsvFilesGlob: generic*.csv
# Before group comment
myAmeriaMyAccounts:
  Account1: USD # List element comment
  # Between list elements comment
  Account2: AMD
detailedOutput: true
categorizeMode: false
monthStartDayNumber: 1
timeZoneLocation: America/New_York
groupAllUnknownTransactions: true
groups:
  # Before group comment
  g1:
    substrings:
      - Sub1 # Group element comment
      # Before group element comment
      - Sub2
`

	// Act
	actualContent := readUseWriteConfig(t, initialContent)

	// Assert
	assertStringEqual(t, actualContent, expectedContent)
}

// FYI: don't need to update this test because input can't change.
func TestWriteToFile_FromOldToNew(t *testing.T) {
	// Arrange.
	initialContent := `# Root comment
inecobankStatementXmlFilesGlob: '*.xml'  # After line comment
inecobankStatementXlsxFilesGlob: "*.xlsx"
ameriaCsvFilesGlob: '*.csv'
myAmeriaAccountStatementXlsxFilesGlob: '*.xls'
myAmeriaHistoryXlsFilesGlob: "1324657890123456"
# Before group comment
myAmeriaMyAccounts:
  Account1: USD # List element comment
  # Between list elements comment
  Account2: AMD
detailedOutput: true
categorizeMode: false
monthStartDayNumber: 1
timeZoneLocation: America/New_York
groupAllUnknownTransactions: true
groupNamesToSubstrings:
  # Before group comment
  g1:
    - Sub1 # Group element comment
    - Sub2
`
	// Note that comments are preserved with following limitations:
	// - Optional fields will be added with default values.
	// - Order of fields is replaced with `Config` struct fields order.
	// - 2 spaces before comments are changed to 1.
	// - Single quotes are changed to double quotes.
	expectedContent := `# Root comment
inecobankStatementXmlFilesGlob: '*.xml' # After line comment
inecobankStatementXlsxFilesGlob: '*.xlsx'
ameriaCsvFilesGlob: '*.csv'
myAmeriaAccountStatementXlsxFilesGlob: '*.xls'
myAmeriaHistoryXlsFilesGlob: "1324657890123456"
# Before group comment
myAmeriaMyAccounts:
  Account1: USD # List element comment
  # Between list elements comment
  Account2: AMD
detailedOutput: true
categorizeMode: false
monthStartDayNumber: 1
timeZoneLocation: America/New_York
groupAllUnknownTransactions: true
groups:
  # Before group comment
  g1:
    substrings:
      - Sub1 # Group element comment
      - Sub2
`

	// Act
	actualContent := readUseWriteConfig(t, initialContent)

	// Assert
	assertStringEqual(t, actualContent, expectedContent)
}

// createTempFileWithContent creates a temporary file with the given content.
func createTempFileWithContent(content string) *os.File {
	tempFile, err := os.CreateTemp("", "test_config_*.yaml")
	if err != nil {
		panic(err)
	}
	if _, err := tempFile.WriteString(content); err != nil {
		panic(err)
	}
	return tempFile
}
