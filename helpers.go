package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// getAbsolutePath checks if a file exists and returns its absolute path.
func getAbsolutePath(filename string) (string, error) {
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return filename, fmt.Errorf("error getting absolute path: %v", err)
	}

	_, err = os.Stat(absPath)
	if os.IsNotExist(err) {
		return absPath, fmt.Errorf("file does not exist: %v", absPath)
	} else if err != nil {
		return absPath, fmt.Errorf("error checking file: %v", err)
	}

	return absPath, nil
}

func getFilesByGlob(glob string) ([]string, error) {
	files, err := filepath.Glob(glob)
	if err != nil {
		return nil, err
	}
	return files, nil
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
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		return err
	}
	return nil
}

func fatalError(err error, inFile bool, openFile bool) {
	if inFile {
		writeAndOpenFile(resultFilePath, err.Error(), openFile)
	}
	log.Fatalf("%s", err)
}

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

// parseTransactionsOfOneType parses transactions from files of one type by one glob pattern.
// Updates parsingWarnings slice with warnings were found.
func parseTransactionsOfOneType(
	glob, nameOfFilesUnderGlob string,
	parser FileParser,
	parsingWarnings *[]string,
) ([]Transaction, error) {
	transactions, warning, err := parseTransactionFiles(glob, parser)
	if err != nil {
		return nil, fmt.Errorf("error on parsing transactions from %s files: %w", nameOfFilesUnderGlob, err)
	}
	if warning != "" {
		*parsingWarnings = append(*parsingWarnings, fmt.Sprintf("Can't parse all %s files: %s", nameOfFilesUnderGlob, warning))
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
		log.Printf("Parsing '%s' with %T%+v parser.", file, parser, parser)
		rawTransactions, err := parser.ParseRawTransactionsFromFile(file)
		if err != nil {
			notFatalError = fmt.Sprintf("Can't parse transactions from '%s' file: %#v", file, err)
			if len(rawTransactions) < 1 {
				// If both error and no transactions then treat error as fatal.
				return result, "", fmt.Errorf("can't parse transactions from '%s' file: %w", file, err)
			} else {
				// Otherwise just log.
				log.Println(notFatalError)
			}
		}
		if len(rawTransactions) < 1 {
			notFatalError = fmt.Sprintf("Can't find transactions in '%s' file.", file)
			log.Println(notFatalError)
		}
		log.Printf("Found %d transactions in '%s' file.", len(rawTransactions), file)
		result = append(result, rawTransactions...)
	}
	return result, notFatalError, nil
}
