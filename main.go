package main

import (
	"embed"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
)

var devMode bool = os.Getenv("DEV_MODE") != "" && os.Getenv("DEV_MODE") != "false"

//go:embed config.yaml
var defaultConfig []byte

//go:embed locales
var locales embed.FS
var i18n *I18n
var langToLocale = map[string]string{
	"en": "en-US",
	"ru": "ru-RU",
}

func init() {
	i18n = &I18n{}
	err := i18n.Init(I18nFsBackend{FS: locales}, "en-US", devMode)
	if err != nil {
		log.Fatalf("Error initializing i18n: %v", err)
	}
}

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
	return i18n.T("AM-Budget-View is a local tool to investigate your expenses and incomes by bank transactions.")
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

	// Check if the config file exists, if not create it with default path and content.
	if _, err := os.Stat(args.ConfigPath); os.IsNotExist(err) {
		args.ConfigPath = DEFAULT_CONFIG_FILE_PATH
		err = os.WriteFile(args.ConfigPath, defaultConfig, 0644)
		if err != nil {
			log.Fatalf("Error creating default config file: %v", err)
		}
		log.Printf("Created default config file at '%s'", args.ConfigPath)
	}

	// Validate ResultMode
	switch args.ResultMode {
	case OPEN_MODE_NONE, OPEN_MODE_WEB, OPEN_MODE_FILE:
		// Valid modes
	default:
		log.Fatalf("Invalid ResultMode '%s', supported only: %s, %s, %s", args.ResultMode, OPEN_MODE_NONE, OPEN_MODE_WEB, OPEN_MODE_FILE)
	}

	// Prepare flags for writing to file and opening file with result.
	isWriteToFile := !args.DontBuildTextReport
	isOpenFileWithResult := args.ResultMode == OPEN_MODE_FILE

	// Parse configuration.
	config, err := readConfig(args.ConfigPath)
	if err != nil {
		fatalError(
			fmt.Errorf("configuration file '%s' is wrong: %w", args.ConfigPath, err),
			isWriteToFile,
			isOpenFileWithResult,
		)
	}

	// Ensure we're running in a terminal window before doing anything else.
	if config.EnsureTerminal {
		ensureTerminalWindow()
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

	// Set language.
	if config.Language != "" {
		i18n.SetLocale(langToLocale[config.Language])
	}

	// Log settings.
	log.Println(i18n.T("Using configuration", "config", config))

	// Parse files to unified Transaction-s.
	// Ineco XML
	transactions := make([]Transaction, 0)
	parsingWarnings := []string{}
	inecoXmlTransactions, err := parseTransactionsOfOneType(
		config.InecobankStatementXmlFilesGlob,
		"Inecobank XML statements",
		InecoXmlParser{},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(err, isWriteToFile, isOpenFileWithResult)
	}
	transactions = append(transactions, inecoXmlTransactions...)

	// Ineco XLSX
	inecoXlsxTransactions, err := parseTransactionsOfOneType(
		config.InecobankStatementXlsxFilesGlob,
		"Inecobank XLSX statements",
		InecoExcelFileParser{},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(err, isWriteToFile, isOpenFileWithResult)
	}
	transactions = append(transactions, inecoXlsxTransactions...)

	// MyAmeria Excel account statements and history.
	myAmeriaStatementsXlsTransactions, err := parseTransactionsOfOneType(
		config.MyAmeriaAccountStatementXlsFilesGlob,
		"MyAmeria Account Statements Excel",
		MyAmeriaExcelStmtFileParser{},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(err, isWriteToFile, isOpenFileWithResult)
	}
	transactions = append(transactions, myAmeriaStatementsXlsTransactions...)
	myAmeriaHistoryXlsTransactions, err := parseTransactionsOfOneType(
		config.MyAmeriaHistoryXlsFilesGlob,
		"MyAmeria History Excel",
		MyAmeriaExcelFileParser{
			MyAccounts: config.MyAmeriaMyAccounts,
		},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(err, isWriteToFile, isOpenFileWithResult)
	}
	transactions = append(transactions, myAmeriaHistoryXlsTransactions...)

	// Ameria CSV
	ameriaCsvTransactions, err := parseTransactionsOfOneType(
		config.AmeriaCsvFilesGlob,
		"Ameria CSV",
		AmeriaCsvFileParser{},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(err, isWriteToFile, isOpenFileWithResult)
	}
	transactions = append(transactions, ameriaCsvTransactions...)

	// Check we found something.
	if len(transactions) < 1 {
		fatalError(
			errors.New(
				i18n.T("can't find transactions, parsing warnings w", "w", parsingWarnings),
			),
			isWriteToFile,
			isOpenFileWithResult,
		)
	}
	log.Println(i18n.T("Total found n transactions", "n", len(transactions)))

	// Create categorization and data handler.
	categorization, err := NewCategorization(config)
	if err != nil {
		fatalError(err, isWriteToFile, isOpenFileWithResult)
	}
	dataHandler := NewDataHandler(config, timeZone, categorization, transactions)

	// Just show uncategorized transactions if in "CategorizeMode" and not WEB mode.
	if config.CategorizeMode && args.ResultMode != OPEN_MODE_WEB {
		err = dataHandler.Categorization.PrintUncategorizedTransactions(dataHandler.Transactions)
		if err != nil {
			fatalError(err, isWriteToFile, isOpenFileWithResult)
		}
		return
	}

	// Build journal entries.
	journalEntries, err := dataHandler.GetJournalEntries(false)
	if err != nil {
		fatalError(errors.New(i18n.T("can't build journal entries", "err", err)), isWriteToFile, isOpenFileWithResult)
	}

	// Produce Beancount file if not disabled.
	if !args.DontBuildBeanconFile {
		transLen, err := buildBeancountFile(journalEntries, dataHandler.Currencies, dataHandler.Accounts, RESULT_BEANCOUNT_FILE_PATH)
		if err != nil {
			fatalError(errors.New(i18n.T("can't build Beancount report", "err", err)), isWriteToFile, isOpenFileWithResult)
		}
		log.Println(i18n.T("Built Beancount file f with n transactions", "file", RESULT_BEANCOUNT_FILE_PATH, "n", transLen))
	}

	// Build statistic.
	monthlyStatistics, err := dataHandler.GetMonthlyStatistics(false)
	if err != nil {
		fatalError(
			errors.New(i18n.T("can't build statistics", "err", err)),
			isWriteToFile,
			isOpenFileWithResult,
		)
	}

	// Produce and show TXT report file if not disabled.
	if !args.DontBuildTextReport {
		var reportStringBuilder strings.Builder
		if len(parsingWarnings) > 0 {
			reportStringBuilder.WriteString("\n - ")
			reportStringBuilder.WriteString(strings.Join(parsingWarnings, "\n - "))
			reportStringBuilder.WriteString("\n\n")
		}

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
				fatalError(errors.New(i18n.T("can't dump interval statistics", "err", err)), isWriteToFile, isOpenFileWithResult)
			}
		}
		fmt.Fprintf(&reportStringBuilder, "\n%s", i18n.T("Total n months", "n", len(monthlyStatistics)))
		result := reportStringBuilder.String()

		// Always print result into logs and conditionally into the file which open through the OS.
		log.Println(result)
		if !args.DontBuildTextReport {
			writeAndOpenFile(RESULT_FILE_PATH, result, isOpenFileWithResult)
		}
	}

	// Start web server if needed.
	if args.ResultMode == OPEN_MODE_WEB {
		go func() {
			time.Sleep(100 * time.Millisecond) // Give the server a moment to start.
			err := openBrowser("http://localhost:" + WEB_PORT)
			if err != nil {
				log.Println(i18n.T("Failed to open browser", "err", err))
			}
		}()

		log.Println(i18n.T("Starting local web server on urlport", "port", WEB_PORT))
		err := ListenAndServe(dataHandler)
		if err != nil {
			fatalError(
				errors.New(i18n.T("failed to start web server, probably app is already running", "err", err)),
				isWriteToFile,
				isOpenFileWithResult,
			)
		}
	}
}

type DataHandler struct {
	Config                    *Config
	TimeZone                  *time.Location
	Categorization            *Categorization
	Transactions              []Transaction
	Accounts                  map[string]*AccountFromTransactions
	Currencies                map[string]*CurrencyStatistics
	journalEntries            []JournalEntry
	uncategorizedTransactions []Transaction
	groupExtractorFactory     StatisticBuilderFactory
	monthlyStatistics         []map[string]*IntervalStatistic
}

func NewDataHandler(config *Config, timeZone *time.Location, categorization *Categorization, transactions []Transaction) *DataHandler {
	return &DataHandler{
		Config:         config,
		TimeZone:       timeZone,
		Categorization: categorization,
		Transactions:   transactions,
	}
}

// GetJournalEntries returns journal entries.
// If isReadFromCache is true and journalEntries are already built, returns them from cache.
// Otherwise builds journal entries and returns them.
// Note that it also builds accounts, currencies and uncategorized transactions.
func (dh *DataHandler) GetJournalEntries(isReadFromCache bool) ([]JournalEntry, error) {
	if isReadFromCache && len(dh.journalEntries) > 0 {
		return dh.journalEntries, nil
	}

	// Build journal entries.
	err := error(nil)
	dh.journalEntries, dh.Accounts, dh.Currencies, dh.uncategorizedTransactions, err = buildJournalEntries(dh.Transactions, dh.Categorization, dh.Config)
	if err != nil {
		return nil, err
	}
	return dh.journalEntries, nil
}

func (dh *DataHandler) GetMonthlyStatistics(isReadFromCache bool) ([]map[string]*IntervalStatistic, error) {
	if isReadFromCache && len(dh.monthlyStatistics) > 0 {
		return dh.monthlyStatistics, nil
	}

	// Create statistic builder factory.
	err := error(nil)
	dh.groupExtractorFactory, err = NewStatisticBuilderByCategories(dh.Accounts)
	if err != nil {
		return nil, err
	}

	// Get journal entries.
	journalEntries, err := dh.GetJournalEntries(true)
	if err != nil {
		return nil, err
	}

	// Build monthly statistics.
	dh.monthlyStatistics, err = BuildMonthlyStatistics(
		journalEntries,
		dh.groupExtractorFactory,
		dh.Config.MonthStartDayNumber,
		dh.TimeZone,
	)
	if err != nil {
		return nil, err
	}
	return dh.monthlyStatistics, nil
}

func (dh *DataHandler) GetUncategorizedTransactions() []Transaction {
	return dh.uncategorizedTransactions
}
