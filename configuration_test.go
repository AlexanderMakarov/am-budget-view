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
acbaRegularAccountXlsFilesGlob: "AcbaAccount*.xls"
acbaCardXlsFilesGlob: "AcbaCard*.xls"
genericCsvFilesGlob: "generic*.csv"
myAmeriaMyAccounts: 
  "Account1": "USD"
  "Account2": "AMD"
detailedOutput: true
categorizeMode: false
monthStartDayNumber: 1
timeZoneLocation: "America/New_York"
groupAllUnknownTransactions: true
groups:
  g1:
    substrings:
      - Sub1
      - Sub2
  g2:
    substrings:
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
	if cfg.ArdshinbankXlsxFilesGlob != "STATEMENT_*.xlsx" {
		t.Errorf(
			"Expected ArdshinbankCsvFilesGlob to be 'STATEMENT_*.xlsx', got '%s'",
			cfg.ArdshinbankXlsxFilesGlob,
		)
	}
	if cfg.AcbaRegularAccountXlsFilesGlob != "AcbaAccount*.xls" {
		t.Errorf(
			"Expected AcbaRegularAccountXlsFilesGlob to be 'AcbaAccount*.xls', got '%s'",
			cfg.AcbaRegularAccountXlsFilesGlob,
		)
	}
	if cfg.AcbaCardXlsFilesGlob != "AcbaCard*.xls" {
		t.Errorf(
			"Expected AcbaCardXlsFilesGlob to be 'AcbaCard*.xls', got '%s'",
			cfg.AcbaCardXlsFilesGlob,
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
	if len(cfg.Groups) != 2 || len(cfg.Groups["g1"].Substrings) != 2 || cfg.Groups["g1"].Substrings[0] != "Sub1" || cfg.Groups["g1"].Substrings[1] != "Sub2" || len(cfg.Groups["g2"].Substrings) != 1 || cfg.Groups["g2"].Substrings[0] != "Sub3" {
		t.Errorf(
			"Expected Groups to have correct mappings, got '%v'",
			cfg.Groups,
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
groups:
  g1:
    substrings:
      - Sub1
      - Sub2
  g2:
    substrings:
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
	// Arrange. Note that "groups" is not specified.
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
`,
	)
	defer os.Remove(tempFile.Name())

	// Act
	_, err := readConfig(tempFile.Name())

	// Assert
	if err == nil {
		t.Fatal("Expected error, but got no error")
	}
	checkErrorContainsSubstring(t, err, "'groups' must be set")
}

func TestReadConfig_MistypedFields(t *testing.T) {
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
groupUnknownTransactions: true  # Should be groupAllUnknownTransactions
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
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}
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
		"'groups' must be set",
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
groups:
  g1:
    substrings:
      - Sub1
      - Sub2
  g2:
    substrings:
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
	if cfg.UIPort != 8080 {
		t.Errorf("Expected UIPort to be 8080, got '%d'", cfg.UIPort)
	}
	tzname, _ := tzlocal.RuntimeTZ()
	if cfg.TimeZoneLocation != tzname {
		t.Errorf("Expected TimeZoneLocation to be '%s', got '%s'", tzname, cfg.TimeZoneLocation)
	}
	if !cfg.GroupAllUnknownTransactions {
		t.Error("Expected GroupAllUnknownTransactions to be true")
	}
	if len(cfg.Groups) != 2 || len(cfg.Groups["g1"].Substrings) != 2 || cfg.Groups["g1"].Substrings[0] != "Sub1" || cfg.Groups["g1"].Substrings[1] != "Sub2" || len(cfg.Groups["g2"].Substrings) != 1 || cfg.Groups["g2"].Substrings[0] != "Sub3" {
		t.Errorf(
			"Expected Groups to have correct mappings, got '%v'",
			cfg.Groups,
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
	// - 2 spaces before comments are changed to 1.
	// - Single quotes are changed to double quotes.
	expectedContent := `uiPort: 8080
# Root comment
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
minCurrencyTimespanPercent: 80
maxCurrencyTimespanGapDays: 30
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
