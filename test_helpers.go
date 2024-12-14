package main

import (
	"strings"
	"testing"
)

func assertStringEqual(t *testing.T, actual, expected string) {
	// Compare actual vs expected strings line by line
	actualLines := strings.Split(actual, "\n")
	expectedLines := strings.Split(expected, "\n")

	if len(actualLines) != len(expectedLines) {
		t.Errorf("Output has different number of lines - got %d, expected %d\n",
			len(actualLines), len(expectedLines))
	}
	linesCount := len(actualLines)
	if len(expectedLines) < linesCount {
		linesCount = len(expectedLines)
	}

	for i := 0; i < linesCount; i++ {
		if actualLines[i] != expectedLines[i] {
			// Find first differing character
			minLen := len(actualLines[i])
			if len(expectedLines[i]) < minLen {
				minLen = len(expectedLines[i])
			}

			diffPos := 0
			for diffPos < minLen && actualLines[i][diffPos] == expectedLines[i][diffPos] {
				diffPos++
			}

			t.Errorf("Line %d differs at position %d:\nExpected: %s\n  Actual: %s\n",
				i+1, diffPos,
				expectedLines[i],
				actualLines[i])
		}
	}
}

func checkErrorContainsSubstring(t *testing.T, err error, substring string) {
	if !strings.Contains(err.Error(), substring) {
		t.Errorf(
			"Expected error message to contain '%s', got '%s'",
			substring,
			err.Error(),
		)
	}
}
