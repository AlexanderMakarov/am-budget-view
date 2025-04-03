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

var devMode bool = os.Getenv("DEV_MODE") != "" && strings.ToLower(os.Getenv("DEV_MODE")) != "false"

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
	log.Printf("Version=%s, devMode=%t", Version, devMode)
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

	var allFileInfos []FileInfo

	// Parse files to unified Transaction-s.
	// Ineco XML
	transactions := make([]Transaction, 0)
	parsingWarnings := []string{}
	inecoXmlTransactions, fileInfos, err := parseTransactionsOfOneType(
		config.InecobankStatementXmlFilesGlob,
		"Inecobank XML statements",
		InecoXmlParser{},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(err, isWriteToFile, isOpenFileWithResult)
	}
	transactions = append(transactions, inecoXmlTransactions...)
	allFileInfos = append(allFileInfos, fileInfos...)

	// Ineco XLSX
	inecoXlsxTransactions, fileInfos, err := parseTransactionsOfOneType(
		config.InecobankStatementXlsxFilesGlob,
		"Inecobank XLSX statements",
		InecoExcelFileParser{},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(err, isWriteToFile, isOpenFileWithResult)
	}
	transactions = append(transactions, inecoXlsxTransactions...)
	allFileInfos = append(allFileInfos, fileInfos...)

	// MyAmeria Excel account statements and history.
	myAmeriaStatementsXlsTransactions, fileInfos, err := parseTransactionsOfOneType(
		config.MyAmeriaAccountStatementXlsFilesGlob,
		"MyAmeria Account Statements Excel",
		MyAmeriaExcelStmtFileParser{},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(err, isWriteToFile, isOpenFileWithResult)
	}
	transactions = append(transactions, myAmeriaStatementsXlsTransactions...)
	myAmeriaHistoryXlsTransactions, fileInfos, err := parseTransactionsOfOneType(
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
	allFileInfos = append(allFileInfos, fileInfos...)

	// Ameria CSV
	ameriaCsvTransactions, fileInfos, err := parseTransactionsOfOneType(
		config.AmeriaCsvFilesGlob,
		"Ameria CSV",
		AmeriaCsvFileParser{},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(err, isWriteToFile, isOpenFileWithResult)
	}
	transactions = append(transactions, ameriaCsvTransactions...)
	allFileInfos = append(allFileInfos, fileInfos...)

	// Generic CSV
	genericCsvTransactions, fileInfos, err := parseTransactionsOfOneType(
		config.GenericCsvFilesGlob,
		"Generic CSV",
		GenericCsvFileParser{},
		&parsingWarnings,
	)
	if err != nil {
		fatalError(err, isWriteToFile, isOpenFileWithResult)
	}
	transactions = append(transactions, genericCsvTransactions...)
	allFileInfos = append(allFileInfos, fileInfos...)

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

	// Create initial Categorization.
	categorization, err := NewCategorization(config)
	if err != nil {
		fatalError(err, isWriteToFile, isOpenFileWithResult)
	}

	// Just show uncategorized transactions if in "CategorizeMode" and not WEB mode.
	if config.CategorizeMode && args.ResultMode != OPEN_MODE_WEB {
		err = categorization.PrintUncategorizedTransactions(transactions)
		if err != nil {
			fatalError(err, isWriteToFile, isOpenFileWithResult)
		}
		return
	}

	// Create data handler.
	dataMart, err := BuildDataMart(transactions, config)
	if err != nil {
		fatalError(err, isWriteToFile, isOpenFileWithResult)
	}
	groupExtractorFactory, err := NewStatisticBuilderByCategories(dataMart.Accounts)
	if err != nil {
		fatalError(err, isWriteToFile, isOpenFileWithResult)
	}
	dataHandler := NewDataHandler(args.ConfigPath, config, timeZone, dataMart, groupExtractorFactory, categorization, allFileInfos)

	// Build journal entries.
	journalEntries, err := dataHandler.GetJournalEntries()
	if err != nil {
		fatalError(errors.New(i18n.T("can't build journal entries", "err", err)), isWriteToFile, isOpenFileWithResult)
	}

	// Produce Beancount file if not disabled.
	if !args.DontBuildBeanconFile {
		transLen, err := buildBeancountFile(journalEntries, dataMart.AllCurrencies, dataMart.Accounts, RESULT_BEANCOUNT_FILE_PATH)
		if err != nil {
			fatalError(errors.New(i18n.T("can't build Beancount report", "err", err)), isWriteToFile, isOpenFileWithResult)
		}
		log.Println(i18n.T("Built Beancount file f with n transactions", "file", RESULT_BEANCOUNT_FILE_PATH, "n", transLen))
	}

	// Build statistic.
	monthlyStatistics, err := dataHandler.GetMonthlyStatistics()
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

// FileInfo represents information about a parsed transaction file.
type FileInfo struct {
	Path              string    `json:"path"`
	Type              string    `json:"type"`
	TransactionsCount int       `json:"transactionsCount"`
	ModifiedTime      time.Time `json:"modifiedTime"`
}

// DataHandler is a handler for data.
// Contians methods to recalculate, cache, persist data.
type DataHandler struct {
	// ConfigPath is a path to the configuration file.
	ConfigPath string
	// Config is a configuration.
	Config *Config
	// TimeZone is a time zone.
	TimeZone *time.Location
	// DataMart is a set of data to build journal entries.
	DataMart *DataMart
	// GroupExtractorFactory is a factory to create statistic builders by categories.
	GroupExtractorFactory StatisticBuilderFactory
	// categorization is a cached struct to categorize transactions.
	categorization *Categorization
	// journalEntries is a list of cached journal entries.
	journalEntries []JournalEntry
	// uncategorizedTransactions is a list of cached uncategorized transactions.
	uncategorizedTransactions []Transaction
	// monthlyStatistics is a list of cached monthly statistics.
	monthlyStatistics []map[string]*IntervalStatistic
	// FileInfos is a list of cached file information.
	FileInfos []FileInfo
}

func NewDataHandler(configPath string, initialConfig *Config, timeZone *time.Location, dataMart *DataMart, groupExtractorFactory StatisticBuilderFactory, initialCategorization *Categorization, fileInfos []FileInfo) *DataHandler {
	return &DataHandler{
		ConfigPath:            configPath,
		Config:                initialConfig,
		TimeZone:              timeZone,
		DataMart:              dataMart,
		GroupExtractorFactory: groupExtractorFactory,
		categorization:        initialCategorization,
		FileInfos:             fileInfos,
	}
}

func (dh *DataHandler) rebuildJournalEntriesAndUncategorizedTransactions() error {
	var err error
	if dh.categorization == nil {
		dh.categorization, err = NewCategorization(dh.Config)
		if err != nil {
			return err
		}
	}
	dh.journalEntries, dh.uncategorizedTransactions, err = buildJournalEntries(dh.DataMart, dh.categorization)
	if err != nil {
		return err
	}
	return nil
}

// GetJournalEntries returns journal entries.
// If isReadFromCache is true and journalEntries are already built, returns them from cache.
// Otherwise builds journal entries and returns them.
// Note that it also builds accounts, currencies and uncategorized transactions.
func (dh *DataHandler) GetJournalEntries() ([]JournalEntry, error) {
	if dh.journalEntries == nil {
		err := dh.rebuildJournalEntriesAndUncategorizedTransactions()
		if err != nil {
			return nil, err
		}
	}
	return dh.journalEntries, nil
}

func (dh *DataHandler) GetUncategorizedTransactions() ([]Transaction, error) {
	if dh.uncategorizedTransactions == nil {
		err := dh.rebuildJournalEntriesAndUncategorizedTransactions()
		if err != nil {
			return nil, err
		}
	}
	return dh.uncategorizedTransactions, nil
}

func (dh *DataHandler) rebuildMonthlyStatistics() error {
	var err error
	journalEntries, err := dh.GetJournalEntries()
	if err != nil {
		return err
	}
	dh.monthlyStatistics, err = BuildMonthlyStatistics(
		journalEntries,
		dh.GroupExtractorFactory,
		dh.Config.MonthStartDayNumber,
		dh.TimeZone,
	)
	if err != nil {
		return err
	}
	return nil
}

func (dh *DataHandler) GetMonthlyStatistics() ([]map[string]*IntervalStatistic, error) {
	if dh.monthlyStatistics == nil {
		err := dh.rebuildMonthlyStatistics()
		if err != nil {
			return nil, err
		}
	}
	return dh.monthlyStatistics, nil
}

func (dh *DataHandler) UpdateGroups(groups map[string]*GroupConfig) error {
	dh.Config.Groups = groups
	err := dh.Config.writeToFile(dh.ConfigPath)
	if err != nil {
		return err
	}
	// Clear caches.
	dh.categorization = nil
	dh.journalEntries = nil
	dh.uncategorizedTransactions = nil
	dh.monthlyStatistics = nil
	return nil
}
