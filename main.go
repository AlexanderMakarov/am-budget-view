package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
)

type Args struct {
	ConfigPath           string `arg:"positional" help:"Path to the configuration YAML file. By default is used 'config.yaml' path."`
	DontOpenFile         bool   `arg:"-n" help:"Flag to don't open result file in OS at the end, only print in STDOUT."`
	DontBuildBeanconFile bool   `arg:"--no-beancount" help:"Flag to don't build Beancount file."`
	DontBuildTextReport  bool   `arg:"--no-txt-report" help:"Flag to don't build TXT file report."`
}

type FileParser interface {
	// ParseRawTransactionsFromFile parses raw/unified transactions
	// from the specified by path file.
	// Returns list of parsed transactions or error if can't parse.
	ParseRawTransactionsFromFile(filePath string) ([]Transaction, error)
}

// Version is application version string and should be updated with `go build -ldflags`.
var Version = "development"

const resultFilePath = "Bank Aggregated Statement.txt"
const resultBeancountFilePath = "Bank Aggregated Statement.beancount"

func main() {
	log.Printf("Version: %s", Version)
	configPath := "config.yaml"

	// Parse arguments and set configPath.
	var args Args
	arg.MustParse(&args)
	if args.ConfigPath != "" {
		configPath = args.ConfigPath
	}
	configPath, err := getAbsolutePath(configPath)
	if err != nil {
		fatalError(fmt.Sprintf("Can't find configuration file '%s': %#v\n", configPath, err), true)
	}
	isOpenFileWithResult := !args.DontOpenFile

	// Parse configuration.
	config, err := readConfig(configPath)
	if err != nil {
		fatalError(
			fmt.Sprintf("Configuration file '%s' is wrong: %+v\n", configPath, err),
			isOpenFileWithResult,
		)
	}

	// Parse timezone or set system.
	timeZone, err := time.LoadLocation(config.TimeZoneLocation)
	if err != nil {
		fatalError(
			fmt.Sprintf("Unknown TimeZoneLocation: %#v.\n", config.TimeZoneLocation),
			isOpenFileWithResult,
		)
	}

	// Build groupsExtractor earlier to check for configuration errors.
	groupExtractorFactory, err := NewStatisticBuilderByDetailsSubstrings(
		config.GroupNamesToSubstrings,
		config.GroupAllUnknownTransactions,
		config.IgnoreSubstrings,
	)
	if err != nil {
		fatalError(fmt.Sprintf("Can't create statistic builder: %#v", err), isOpenFileWithResult)
	}

	// Log settings.
	log.Printf("Using configuration: %+v", config)

	// Parse files to raw transactions.
	// Ineco XML
	transactions := make([]Transaction, 0)
	parsingWarnings := []string{}
	inecoXmlTransactions, err := parseTransactionsOfOneType(
		config.InecobankStatementXmlFilesGlob,
		InecoXmlParser{},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(fmt.Sprintf("Can't parse Inecobank XML statements: %#v", err), isOpenFileWithResult)
	}
	transactions = append(transactions, inecoXmlTransactions...)
	// Ineco XLSX
	inecoXlsxTransactions, err := parseTransactionsOfOneType(
		config.InecobankStatementXlsxFilesGlob,
		InecoExcelFileParser{},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(fmt.Sprintf("Can't parse Inecobank XLSX statements: %#v", err), isOpenFileWithResult)
	}
	transactions = append(transactions, inecoXlsxTransactions...)
	// MyAmeria Excel
	myAmeriaXlsTransactions, err := parseTransactionsOfOneType(
		config.MyAmeriaHistoryXlsFilesGlob,
		MyAmeriaExcelFileParser{
			MyAccounts:              config.MyAmeriaMyAccounts,
			DetailsIncomeSubstrings: config.MyAmeriaIncomeSubstrings,
		},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(fmt.Sprintf("Can't parse MyAmeria History: %#v", err), isOpenFileWithResult)
	}
	transactions = append(transactions, myAmeriaXlsTransactions...)
	// MyAmeria CSV
	ameriaCsvTransactions, err := parseTransactionsOfOneType(
		config.AmeriaCsvFilesGlob,
		AmeriaCsvFileParser{config.AmeriaCsvFilesCurrency},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(fmt.Sprintf("Can't parse Ameria in-CSV transactions: %#v", err), isOpenFileWithResult)
	}
	transactions = append(transactions, ameriaCsvTransactions...)
	// Check we found something.
	if len(transactions) < 1 {
		fatalError(
			fmt.Sprintf(
				"Can't find transactions, check that '*Glob' configuration parameters matches something and see parsing warnings:\n%s\n",
				strings.Join(parsingWarnings, "\n"),
			),
			isOpenFileWithResult,
		)
	}
	log.Printf("Total found %d transactions.", len(transactions))

	// Show uncategorized transactions if in "CategorizeMode".
	if config.CategorizeMode {
		err := PrintUncategorizedTransactions(transactions, config)
		if err != nil {
			log.Fatalf("Can't check for uncategorized transactions: %#v", err)
		}
		return
	}

	// Produce Beancount file if not disabled.
	if !args.DontBuildBeanconFile {
		transLen, err := buildBeanconFile(transactions, config, resultBeancountFilePath)
		if err != nil {
			log.Fatalf("Can't build Beancount report: %#v", err)
		}
		log.Printf("Built Beancount file '%s' with %d transactions.", resultBeancountFilePath, transLen)
	}

	// Produce and show TXT report file if not disabled.
	if !args.DontBuildTextReport {

		// Build statistic.
		statistics, err := BuildMonthlyStatistic(
			transactions,
			groupExtractorFactory,
			config.MonthStartDayNumber,
			timeZone,
		)
		if err != nil {
			fatalError(fmt.Sprintf("Can't build statistic: %#v", err), isOpenFileWithResult)
		}

		// Process received statistics.
		result := strings.Join(parsingWarnings, "\n")
		for _, s := range statistics {
			if config.DetailedOutput {
				result = result + "\n" + s.String()
				continue
			}

			// Note that this logic is intentionally separated from `func (s *IntervalStatistic) String()`.
			income := MapOfGroupsToString(s.Income)
			expense := MapOfGroupsToString(s.Expense)
			result = result + "\n" + fmt.Sprintf(
				"\n%s..%s:\n  Income (%d, sum=%s):%s\n  Expenses (%d, sum=%s):%s",
				s.Start.Format(OutputDateFormat),
				s.End.Format(OutputDateFormat),
				len(income),
				MapOfGroupsSum(s.Income),
				strings.Join(income, ""),
				len(s.Expense),
				MapOfGroupsSum(s.Expense),
				strings.Join(expense, ""),
			)
		}
		result = fmt.Sprintf("%s\nTotal %d months.", result, len(statistics))

		// Always print result into logs and conditionally into the file which open through the OS.
		log.Print(result)
		if !args.DontOpenFile { // Twice no here, but we need in good default value for the flag and too lazy.
			writeAndOpenFile(resultFilePath, result)
		}
	}
}

func fatalError(err string, inFile bool) {
	if inFile {
		writeAndOpenFile(resultFilePath, err)
	}
	log.Fatalf(err)
}

func writeAndOpenFile(resultFilePath, content string) {
	if err := os.WriteFile(resultFilePath, []byte(content), 0644); err != nil {
		log.Fatalf("Can't write result file into %s: %#v", resultFilePath, err)
	}
	if err := openFileInOS(resultFilePath); err != nil {
		log.Fatalf("Can't open result file %s: %#v", resultFilePath, err)
	}
}

// parseTransactionFiles parses transactions from files of one type by one glob pattern.
// Updates parsingWarnings slice warnings were found.
func parseTransactionsOfOneType(
	glob string,
	parser FileParser,
	parsingWarnings *[]string,
) ([]Transaction, error) {
	transactions, warning, err := parseTransactionFiles(glob, parser)
	if err != nil {
		return nil, fmt.Errorf("can't parse transactions: %#v", err)
	}
	if warning != "" {
		*parsingWarnings = append(*parsingWarnings, fmt.Sprintf("Parsing warning: %s", warning))
	}
	return transactions, nil
}

// parseTransactionFiles parses transactions from files by glob pattern.
// Returns list of transactions, not fatal error message and error if it is fatal.
func parseTransactionFiles(glog string, parser FileParser) ([]Transaction, string, error) {
	files, err := getFilesByGlob(glog)
	if err != nil {
		return nil, "", err
	}

	result := make([]Transaction, 0)
	notFatalError := ""
	for _, file := range files {
		log.Printf("Parsing '%s' with %+v parser.", file, parser)
		rawTransactions, err := parser.ParseRawTransactionsFromFile(file)
		if err != nil {
			notFatalError = fmt.Sprintf("Can't parse transactions from '%s' file: %#v", file, err)
			if len(rawTransactions) < 1 {
				// If both error and no transactions then treat error as fatal.
				return result, "", err
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
