package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
)

const resultFilePath = "AM Budget View.txt"
const resultBeancountFilePath = "AM Budget View.beancount"
const OPEN_MODE_NONE = "none"
const OPEN_MODE_WEB = "web"
const OPEN_MODE_FILE = "file"

type Args struct {
	ConfigPath           string `arg:"positional" default:"config.yaml" help:"Path to the configuration YAML file. By default is used 'config.yaml' path."`
	ResultMode           string `arg:"-o" default:"web" help:"Specify how to open the result: 'none' for print into STDOUT only, 'web' for web server to see in browser, 'file' for opening result file in OS." enum:"none,web,file"`
	DontBuildBeanconFile bool   `arg:"--no-beancount" help:"Flag to don't build Beancount file."`
	DontBuildTextReport  bool   `arg:"--no-txt-report" help:"Flag to don't build TXT file report."`
}

// Version is application version string and should be updated with `go build -ldflags`.
var Version = "development"

func (Args) Version() string {
	return Version
}

func (Args) Description() string {
	return "AM-Budget-View is a local tool to investigate your expenses and incomes by bank transactions."
}

func main() {
	log.Printf("Version: %s", Version)

	// Parse arguments and set configPath.
	var args Args
	p, err := arg.NewParser(arg.Config{}, &args)
	if err != nil {
		log.Fatalf("Error creating argument parser: %v", err)
	}

	err = p.Parse(os.Args[1:])
	if err != nil {
		// Check if the error is a help request
		if err == arg.ErrHelp {
			p.WriteHelp(os.Stdout)
			os.Exit(0)
		}
		log.Fatalf("Error parsing arguments: %v", err)
	}

	// Validate ResultMode
	switch args.ResultMode {
	case OPEN_MODE_NONE, OPEN_MODE_WEB, OPEN_MODE_FILE:
		// Valid modes
	default:
		log.Fatalf("Invalid ResultMode '%s', supported only: %s, %s, %s", args.ResultMode, OPEN_MODE_NONE, OPEN_MODE_WEB, OPEN_MODE_FILE)
	}

	configPath, err := getAbsolutePath(args.ConfigPath)
	if err != nil {
		fatalError(fmt.Errorf("can't find configuration file '%s': %v", args.ConfigPath, err), true, true)
	}
	isWriteToFile := !args.DontBuildTextReport
	isOpenFileWithResult := args.ResultMode == OPEN_MODE_FILE

	// Parse configuration.
	config, err := readConfig(configPath)
	if err != nil {
		fatalError(
			fmt.Errorf("configuration file '%s' is wrong: %w", configPath, err),
			isWriteToFile,
			isOpenFileWithResult,
		)
	}

	// Parse timezone or set system.
	timeZone, err := time.LoadLocation(config.TimeZoneLocation)
	if err != nil {
		fatalError(
			fmt.Errorf("unknown TimeZoneLocation: %s", config.TimeZoneLocation),
			isWriteToFile,
			isOpenFileWithResult,
		)
	}

	// Log settings.
	log.Printf("Using configuration: %+v", config)

	// Parse files to unified Transaction-s.
	// Ineco XML
	transactions := make([]Transaction, 0)
	parsingWarnings := []string{}
	inecoXmlTransactions, err := parseTransactionsOfOneType(
		config.InecobankStatementXmlFilesGlob,
		InecoXmlParser{},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(fmt.Errorf("can't parse Inecobank XML statements: %w", err), isWriteToFile, isOpenFileWithResult)
	}
	transactions = append(transactions, inecoXmlTransactions...)

	// Ineco XLSX
	inecoXlsxTransactions, err := parseTransactionsOfOneType(
		config.InecobankStatementXlsxFilesGlob,
		InecoExcelFileParser{},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(fmt.Errorf("can't parse Inecobank XLSX statements: %w", err), isWriteToFile, isOpenFileWithResult)
	}
	transactions = append(transactions, inecoXlsxTransactions...)

	// MyAmeria Excel account statements and history.
	myAmeriaStatementsXlsTransactions, err := parseTransactionsOfOneType(
		config.MyAmeriaAccountStatementXlsFilesGlob,
		MyAmeriaExcelStmtFileParser{},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(fmt.Errorf("can't parse MyAmeria Account Statements Excel: %w", err), isWriteToFile, isOpenFileWithResult)
	}
	transactions = append(transactions, myAmeriaStatementsXlsTransactions...)
	myAmeriaHistoryXlsTransactions, err := parseTransactionsOfOneType(
		config.MyAmeriaHistoryXlsFilesGlob,
		MyAmeriaExcelFileParser{
			MyAccounts: config.MyAmeriaMyAccounts,
		},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(fmt.Errorf("can't parse MyAmeria History Excel: %w", err), isWriteToFile, isOpenFileWithResult)
	}
	transactions = append(transactions, myAmeriaHistoryXlsTransactions...)

	// Ameria CSV
	ameriaCsvTransactions, err := parseTransactionsOfOneType(
		config.AmeriaCsvFilesGlob,
		AmeriaCsvFileParser{},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(fmt.Errorf("can't parse Ameria CSV: %w", err), isWriteToFile, isOpenFileWithResult)
	}
	transactions = append(transactions, ameriaCsvTransactions...)

	// Check we found something.
	if len(transactions) < 1 {
		fatalError(
			fmt.Errorf(
				"can't find transactions, check that '*Glob' configuration parameters matches something and see parsing warnings:\n%s",
				strings.Join(parsingWarnings, "\n"),
			),
			isWriteToFile,
			isOpenFileWithResult,
		)
	}
	log.Printf("Total found %d transactions.", len(transactions))

	// Show uncategorized transactions if in "CategorizeMode".
	if config.CategorizeMode {
		err := PrintUncategorizedTransactions(transactions, config)
		if err != nil {
			log.Fatalf("can't check for uncategorized transactions: %#v", err)
		}
		return
	}

	// Build journal entries.
	journalEntries, accounts, currencies, err := buildJournalEntries(transactions, config)
	if err != nil {
		fatalError(fmt.Errorf("can't build journal entries: %w", err), isWriteToFile, isOpenFileWithResult)
	}

	// Produce Beancount file if not disabled.
	if !args.DontBuildBeanconFile {
		transLen, err := buildBeancountFile(journalEntries, currencies, accounts, resultBeancountFilePath)
		if err != nil {
			fatalError(fmt.Errorf("can't build Beancount report: %w", err), isWriteToFile, isOpenFileWithResult)
		}
		log.Printf("Built Beancount file '%s' with %d transactions.", resultBeancountFilePath, transLen)
	}

	// Build statistic.
	groupExtractorFactory, err := NewStatisticBuilderByCategories(accounts)
	if err != nil {
		fatalError(
			fmt.Errorf("can't create statistic builder: %w", err),
			isWriteToFile,
			isOpenFileWithResult,
		)
	}
	monthlyStatistics, err := BuildMonthlyStatistics(
		journalEntries,
		groupExtractorFactory,
		config.MonthStartDayNumber,
		timeZone,
	)
	if err != nil {
		fatalError(fmt.Errorf("can't build statistics: %w", err), isWriteToFile, isOpenFileWithResult)
	}

	// Produce and show TXT report file if not disabled.
	if !args.DontBuildTextReport {
		var reportStringBuilder strings.Builder
		reportStringBuilder.WriteString(strings.Join(parsingWarnings, "\n"))

		currency := ""
		if !config.DetailedOutput {
			// For text report use first from ConvertToCurrencies or just first currency.
			if len(config.ConvertToCurrencies) > 0 {
				currency = config.ConvertToCurrencies[0]
			} else {
				currency = journalEntries[0].AccountCurrency
				if currency == "" {
					currency = journalEntries[0].OriginCurrency
				}
			}
		}
		for _, oneMonthStatistics := range monthlyStatistics {
			if err := DumpIntervalStatistics(oneMonthStatistics, &reportStringBuilder, currency, config.DetailedOutput); err != nil {
				log.Fatalf("can't dump interval statistics: %#v", err)
			}
		}
		fmt.Fprintf(&reportStringBuilder, "\nTotal %d months.", len(monthlyStatistics))
		result := reportStringBuilder.String()

		// Always print result into logs and conditionally into the file which open through the OS.
		log.Print(result)
		if !args.DontBuildTextReport {
			writeAndOpenFile(resultFilePath, result, isOpenFileWithResult)
		}
	}

	// Start web server if needed.
	if args.ResultMode == OPEN_MODE_WEB {
		go func() {
			time.Sleep(100 * time.Millisecond) // Give the server a moment to start
			err := openBrowser("http://localhost:8080")
			if err != nil {
				log.Printf("Failed to open browser: %v", err)
			}
		}()
		ListenAndServe(monthlyStatistics, accounts)
	}
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
// Updates parsingWarnings slice warnings were found.
func parseTransactionsOfOneType(
	glob string,
	parser FileParser,
	parsingWarnings *[]string,
) ([]Transaction, error) {
	transactions, warning, err := parseTransactionFiles(glob, parser)
	if err != nil {
		return nil, err
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
		err = fmt.Errorf("unsupported platform")
	}
	return err
}
