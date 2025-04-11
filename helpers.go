package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
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
	errMsg := fmt.Sprintf("ERROR: %s", err)
	if inFile {
		writeAndOpenFile(RESULT_FILE_PATH, errMsg, openFile)
	}
	log.Fatal(errMsg)
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
// Returns list of transactions, list of file infos and error if it is fatal.
func parseTransactionsOfOneType(
	glob,
	nameOfFilesUnderGlob string,
	parser FileParser,
	parsingWarnings *[]string,
) ([]Transaction, []FileInfo, error) {
	transactions, warning, fileInfos, err := parseTransactionFiles(glob, parser)
	if err != nil {
		return nil, nil, errors.New(i18n.T("error on parsing transactions from name files", "name", nameOfFilesUnderGlob, "err", err))
	}
	if warning != "" {
		*parsingWarnings = append(*parsingWarnings, i18n.T("Can't parse all n files", "n", nameOfFilesUnderGlob, "warning", warning))
	}
	return transactions, fileInfos, nil
}

// parseTransactionFiles parses transactions from files by glob pattern.
// Returns list of transactions, not fatal error message and error if it is fatal.
func parseTransactionFiles(glob string, parser FileParser) ([]Transaction, string, []FileInfo, error) {
	files, err := getFilesByGlob(glob)
	if err != nil {
		return nil, "", nil, err
	}
	if len(files) < 1 {
		workingDir, err := os.Getwd()
		if err != nil {
			return nil, "", nil, errors.New(i18n.T("can't get working directory", "err", err))
		}
		return nil, i18n.T("there are no files in d matching p pattern", "d", workingDir, "p", glob), nil, nil
	}

	result := make([]Transaction, 0)
	fileInfos := make([]FileInfo, 0)
	notFatalError := ""

	for _, file := range files {
		log.Println(i18n.T("Parsing file with parser", "file", file, "parser", parser))
		rawTransactions, sourceType, err := parser.ParseRawTransactionsFromFile(file)
		if err != nil {
			notFatalError = i18n.T("can't parse transactions from file f", "f", file, "err", err)
			if len(rawTransactions) < 1 {
				// If both error and no transactions then treat error as fatal.
				return result, "", nil, errors.New(i18n.T("can't parse transactions from file f", "f", file, "err", err))
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

		// Get file info.
		fileInfo, err := os.Stat(file)
		if err != nil {
			return result, "", nil, errors.New(i18n.T("can't get file info for '%s': %v", file, err))
		}
		var fileFromDate, fileToDate time.Time
		if len(rawTransactions) > 0 {
			fileFromDate = rawTransactions[0].Date
			fileToDate = rawTransactions[0].Date

			for _, transaction := range rawTransactions {
				if transaction.Date.Before(fileFromDate) {
					fileFromDate = transaction.Date
				}
				if transaction.Date.After(fileToDate) {
					fileToDate = transaction.Date
				}
			}
		}
		fileInfos = append(fileInfos, FileInfo{
			Path:              file,
			Type:              sourceType,
			TransactionsCount: len(rawTransactions),
			ModifiedTime:      fileInfo.ModTime(),
			FromDate:          fileFromDate,
			ToDate:            fileToDate,
		})
	}

	return result, notFatalError, fileInfos, nil
}
