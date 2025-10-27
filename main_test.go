package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHelper provides utilities for testing
type TestHelper struct {
	tempDir    string
	configFile string
}

// NewTestHelper creates a new test helper with a temporary directory
func NewTestHelper(t *testing.T) *TestHelper {
	tempDir, err := os.MkdirTemp("", "test_aggregate_inecobank_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	return &TestHelper{
		tempDir: tempDir,
	}
}

// Cleanup removes the temporary directory and all its contents
func (th *TestHelper) Cleanup() {
	if th.tempDir != "" {
		os.RemoveAll(th.tempDir)
	}
}

// CreateConfigFile creates a config file in the temp directory with the given content
func (th *TestHelper) CreateConfigFile(content string) (string, error) {
	configFile := filepath.Join(th.tempDir, "test_config.yaml")
	err := os.WriteFile(configFile, []byte(content), 0644)
	if err != nil {
		return "", err
	}
	th.configFile = configFile
	return configFile, nil
}

// CreateMinimalConfig creates a minimal valid config file for testing
func (th *TestHelper) CreateMinimalConfig() (string, error) {
	configContent := `
language: en
timeZoneLocation: "UTC"
ensureTerminal: false
groups:
  income:
    name: "Income"
    color: "#00ff00"
  expense:
    name: "Expense"
    color: "#ff0000"
`
	return th.CreateConfigFile(configContent)
}

// ------------------ TESTS ------------------

func TestParseArgs_Success(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    Args
		isHelpRequested bool
	}{
		{
			name: "default args",
			args: []string{},
			want: Args{
				ConfigPath:           "config.yaml",
				ResultMode:           "web",
				DontBuildBeanconFile: false,
				DontBuildTextReport:  false,
			},
			isHelpRequested: false,
		},
		{
			name: "custom config path",
			args: []string{"custom_config.yaml"},
			want: Args{
				ConfigPath:           "custom_config.yaml",
				ResultMode:           "web",
				DontBuildBeanconFile: false,
				DontBuildTextReport:  false,
			},
			isHelpRequested: false,
		},
		{
			name: "result mode none",
			args: []string{"-o", "none"},
			want: Args{
				ConfigPath:           "config.yaml",
				ResultMode:           "none",
				DontBuildBeanconFile: false,
				DontBuildTextReport:  false,
			},
			isHelpRequested: false,
		},
		{
			name: "result mode file",
			args: []string{"-o", "file"},
			want: Args{
				ConfigPath:           "config.yaml",
				ResultMode:           "file",
				DontBuildBeanconFile: false,
				DontBuildTextReport:  false,
			},
			isHelpRequested: false,
		},
		{
			name: "disable beancount",
			args: []string{"--no-beancount"},
			want: Args{
				ConfigPath:           "config.yaml",
				ResultMode:           "web",
				DontBuildBeanconFile: true,
				DontBuildTextReport:  false,
			},
			isHelpRequested: false,
		},
		{
			name: "disable text report",
			args: []string{"--no-txt-report"},
			want: Args{
				ConfigPath:           "config.yaml",
				ResultMode:           "web",
				DontBuildBeanconFile: false,
				DontBuildTextReport:  true,
			},
			isHelpRequested: false,
		},
		{
			name: "invalid result mode",
			args: []string{"-o", "invalid"},
			want: Args{
				ConfigPath:           "config.yaml",
				ResultMode:           "invalid",
				DontBuildBeanconFile: false,
				DontBuildTextReport:  false,
			},
			isHelpRequested: false,
		},
		{
			name: "help requested",
			args: []string{"--help"},
			isHelpRequested: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, isHelpRequested, err := parseArgs(tt.args)
			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if isHelpRequested != tt.isHelpRequested {
				t.Errorf("Expected isHelpRequested %t, got %t", tt.isHelpRequested, isHelpRequested)
			}
			if got != tt.want {
				t.Errorf("Expected %+v, got %+v", tt.want, got)
			}
		})
	}
}

func TestParseArgs_InvalidArgument(t *testing.T) {
	_, _, err := parseArgs([]string{"--invalid-argument"})
	expectedErrMsg := "unknown argument --invalid-argument"
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("got: %v, expected: %s", err, expectedErrMsg)
	}
}

func TestRunApplication_InvalidResultMode(t *testing.T) {
	helper := NewTestHelper(t)
	defer helper.Cleanup()

	configFile, err := helper.CreateMinimalConfig()
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	args := Args{
		ConfigPath: configFile,
		ResultMode: "invalid",
	}

	err = runApplication(args)
	if err == nil {
		t.Error("Expected error for invalid result mode, got nil")
	}
	if !strings.Contains(err.Error(), "invalid ResultMode") {
		t.Errorf("Expected error about invalid ResultMode, got: %v", err)
	}
}

// TestRunApplication_InvalidConfig tests runApplication with invalid config content
func TestRunApplication_InvalidConfig(t *testing.T) {
	helper := NewTestHelper(t)
	defer helper.Cleanup()

	// Create invalid config file
	configFile, err := helper.CreateConfigFile("invalid: yaml: content: [")
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	args := Args{
		ConfigPath: configFile,
		ResultMode: OPEN_MODE_NONE,
	}

	err = runApplication(args)
	if err == nil {
		t.Error("Expected error for invalid config, got nil")
	}
	if !strings.Contains(err.Error(), "configuration file") {
		t.Errorf("Expected error about configuration file, got: %v", err)
	}
}

// TestRunApplication_InvalidTimezone tests runApplication with invalid timezone
func TestRunApplication_InvalidTimezone(t *testing.T) {
	helper := NewTestHelper(t)
	defer helper.Cleanup()

	configContent := `
timeZoneLocation: "Invalid/Timezone"
groups:
  income:
    name: "Income"
    color: "#00ff00"
  expense:
    name: "Expense"
    color: "#ff0000"
`
	configFile, err := helper.CreateConfigFile(configContent)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	args := Args{
		ConfigPath: configFile,
		ResultMode: OPEN_MODE_NONE,
	}

	err = runApplication(args)
	if err == nil {
		t.Error("Expected error for invalid timezone, got nil")
	}
	if !strings.Contains(err.Error(), "unknown time zone") {
		t.Errorf("Expected error about timezone, got: %v", err)
	}
}

// TestHandleError tests the handleError function
func TestHandleError(t *testing.T) {
	helper := NewTestHelper(t)
	defer helper.Cleanup()

	// Change to temp directory to avoid creating files in the project root
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(helper.tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	testErr := errors.New("test error")

	// Test with file writing enabled
	err = handleError(testErr, true, false)
	if err == nil {
		t.Error("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "ERROR: test error") {
		t.Errorf("Expected error message to contain 'ERROR: test error', got: %v", err)
	}

	// Check if error file was created
	if _, err := os.Stat(RESULT_FILE_PATH); os.IsNotExist(err) {
		t.Error("Expected error file to be created")
	}
}
