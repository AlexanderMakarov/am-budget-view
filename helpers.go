package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// getFilesByGlob retrieves files matching the glob pattern.
func getFilesByGlob(glob string) ([]string, error) {
	files, err := filepath.Glob(glob)
	if err != nil {
		return nil, err
	}
	return files, nil
}

// openBrowser opens the specified URL in the default web browser.
func openBrowser(url string) error {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = errors.New(i18n.T("unsupported platform"))
	}
	return err
}

// fatalError handles fatal errors and logs them.
func fatalError(err error, inFile bool, openFile bool) {
	if inFile {
		writeAndOpenFile(RESULT_FILE_PATH, err.Error(), openFile)
	}
	log.Fatalf("%s", err)
}

// writeAndOpenFile writes content to a file and optionally opens it.
func writeAndOpenFile(resultFilePath, content string, openFile bool) {
	if err := os.WriteFile(resultFilePath, []byte(content), 0644); err != nil {
		log.Fatalf("Can't write result file into %s: %#v", resultFilePath, err)
	}
	if openFile {
		if err := openFileInOS(resultFilePath); err != nil {
			log.Fatalf("Can't open result file %s: %#v", resultFilePath, err)
		}
	}
}

// openFileInOS opens file in OS-specific viewer.
func openFileInOS(url string) error {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = errors.New(i18n.T("unsupported platform"))
	}
	return err
}

// parseTransactionsOfOneType parses transactions from files of one type by one glob pattern.
// Updates parsingWarnings slice with warnings were found.
func parseTransactionsOfOneType(
	glob, nameOfFilesUnderGlob string,
	parser FileParser,
	parsingWarnings *[]string,
) ([]Transaction, error) {
	transactions, warning, err := parseTransactionFiles(glob, parser)
	if err != nil {
		return nil, errors.New(i18n.T("error on parsing transactions from name files", "name", nameOfFilesUnderGlob, "err", err))
	}
	if warning != "" {
		*parsingWarnings = append(*parsingWarnings, i18n.T("Can't parse all n files", "n", nameOfFilesUnderGlob, "warning", warning))
	}
	return transactions, nil
}

// parseTransactionFiles parses transactions from files by glob pattern.
// Returns list of transactions, not fatal error message and error if it is fatal.
func parseTransactionFiles(glob string, parser FileParser) ([]Transaction, string, error) {
	files, err := getFilesByGlob(glob)
	if err != nil {
		return nil, "", err
	}
	if len(files) < 1 {
		workingDir, err := os.Getwd()
		if err != nil {
			return nil, "", fmt.Errorf("can't get working directory: %w", err)
		}
		return nil, fmt.Sprintf("there are no files in '%s' matching '%s' pattern.", workingDir, glob), nil
	}

	result := make([]Transaction, 0)
	notFatalError := ""
	for _, file := range files {
		log.Println(i18n.T("Parsing file with parser", "file", file, "parser", parser))
		rawTransactions, err := parser.ParseRawTransactionsFromFile(file)
		if err != nil {
			notFatalError = i18n.T("can't parse transactions from file f", "f", file, "err", err)
			if len(rawTransactions) < 1 {
				// If both error and no transactions then treat error as fatal.
				return result, "", errors.New(i18n.T("can't parse transactions from file f", "f", file, "err", err))
			} else {
				// Otherwise just log.
				log.Println(notFatalError)
			}
		}
		if len(rawTransactions) < 1 {
			notFatalError = i18n.T("Can't find transactions in f file", "f", file)
			log.Println(notFatalError)
		}
		log.Println(i18n.T("Found n transactions in f file", "n", len(rawTransactions), "f", file))
		result = append(result, rawTransactions...)
	}
	return result, notFatalError, nil
}
