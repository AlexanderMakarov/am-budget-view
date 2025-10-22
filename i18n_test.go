package main

import (
	"embed"
	"testing"
)

//go:embed locales
var testLocales embed.FS

func TestParseCommaSeparatedWithQuotes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple comma separation",
			input:    "a, b, c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "quoted strings with commas",
			input:    "separator: ', '",
			expected: []string{"separator: ', '"},
		},
		{
			name:     "mixed quoted and unquoted",
			input:    "a, separator: ', ', b",
			expected: []string{"a", "separator: ', '", "b"},
		},
		{
			name:     "double quotes",
			input:    `separator: ", "`,
			expected: []string{`separator: ", "`},
		},
		{
			name:     "no commas",
			input:    "single",
			expected: []string{"single"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "nested quotes",
			input:    `separator: "', '"`,
			expected: []string{`separator: "', '"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCommaSeparatedWithQuotes(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d parts, got %d", len(tt.expected), len(result))
				return
			}
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("part %d: expected %q, got %q", i, expected, result[i])
				}
			}
		})
	}
}

func TestI18nTranslation(t *testing.T) {
	backend := I18nFsBackend{FS: testLocales}
	i18n := &I18n{}
	err := i18n.Init(backend, "en-US", false)
	if err != nil {
		t.Fatalf("Failed to initialize i18n: %v", err)
	}

	tests := []struct {
		name     string
		key      string
		args     []interface{}
		expected string
	}{
		{
			name:     "simple interpolation",
			key:      "Total found n transactions",
			args:     []interface{}{"n", 42},
			expected: "Total found 42 transactions.",
		},
		{
			name:     "list formatting with quoted separator",
			key:      "can't build Beancount report, transactions from following sources don't have Reciever/Payer account number: sources",
			args:     []interface{}{"sources", "source1, source2, source3"},
			expected: "can't build Beancount report, transactions from following sources don't have Reciever/Payer account number: source1, source2, source3",
		},
		{
			name:     "list formatting with array",
			key:      "can't build Beancount report, transactions from following sources don't have Reciever/Payer account number: sources",
			args:     []interface{}{"sources", []string{"source1", "source2", "source3"}},
			expected: "can't build Beancount report, transactions from following sources don't have Reciever/Payer account number: source1, source2, source3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := i18n.T(tt.key, tt.args...)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
