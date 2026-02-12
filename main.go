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

	"github.com/AlexanderMakarov/am-budget-view/internal/app"
	"github.com/AlexanderMakarov/am-budget-view/internal/beancount"
	"github.com/AlexanderMakarov/am-budget-view/internal/config"
	"github.com/AlexanderMakarov/am-budget-view/internal/currency"
	"github.com/AlexanderMakarov/am-budget-view/internal/i18n"
	"github.com/AlexanderMakarov/am-budget-view/internal/model"
	"github.com/AlexanderMakarov/am-budget-view/internal/platform"
	"github.com/AlexanderMakarov/am-budget-view/internal/statistic"
	"github.com/AlexanderMakarov/am-budget-view/internal/ui"
)

var devMode bool = os.Getenv("DEV_MODE") != "" && strings.ToLower(os.Getenv("DEV_MODE")) != "false"

//go:embed config.yaml
var defaultConfig []byte

//go:embed locales
var locales embed.FS

//go:embed static
var static embed.FS

//go:embed templates
var templateFS embed.FS

var langToLocale = map[string]string{
	"en": "en-US",
	"ru": "ru-RU",
}

func init() {
	log.Printf("Version=%s, devMode=%t", Version, devMode)
	err := i18n.Init(i18n.I18nFsBackend{FS: locales}, "en-US", devMode)
	if err != nil {
		log.Fatalf("Error initializing i18n: %v", err)
	}
}

type Args struct {
	ConfigPath           string `arg:"positional" default:"config.yaml" help:"Path to the configuration YAML file. By default is used 'config.yaml' path."`
	ResultMode           string `arg:"-o" default:"web" help:"Specify how to open the result: 'none' for print into STDOUT only, 'web' for web server to see in browser, 'file' for opening result file in OS." enum:"none,web,file"`
	DontBuildBeanconFile bool   `arg:"--no-beancount" help:"Flag to don't build Beancount file."`
	DontBuildTextReport  bool   `arg:"--no-txt-report" help:"Flag to don't build TXT file report."`
	NoTerminal           bool   `arg:"--no-terminal" help:"Run in the current terminal without opening a new terminal window. Skips EnsureTerminal even if configured."`
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
	// Parse command line arguments.
	args, isHelpRequested, err := parseArgs(os.Args[1:])
	if err != nil {
		log.Fatalf("Error parsing arguments: %v", err)
	} else if isHelpRequested {
		os.Exit(0)
	}

	// Run the main application logic.
	err = runApplication(args)
	if err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

// parseArgs parses command line arguments.
func parseArgs(args []string) (Args, bool, error) {
	var parsedArgs Args
	p, err := arg.NewParser(arg.Config{}, &parsedArgs)
	if err != nil {
		return Args{}, false, fmt.Errorf("error creating argument parser: %w", err)
	}

	err = p.Parse(args)
	if err != nil {
		// Check if the error is a help request
		if err == arg.ErrHelp {
			p.WriteHelp(os.Stdout)
			return Args{}, true, nil
		}
		return Args{}, false, fmt.Errorf("error parsing arguments: %w", err)
	}

	return parsedArgs, false, nil
}

// runApplication contains the main application logic, separated for testing.
func runApplication(args Args) error {
	// Check if the config file exists, if not create it with default path and content.
	if _, err := os.Stat(args.ConfigPath); os.IsNotExist(err) {
		args.ConfigPath = model.DEFAULT_CONFIG_FILE_PATH
		err = os.WriteFile(args.ConfigPath, defaultConfig, 0644)
		if err != nil {
			return fmt.Errorf("error creating default config file: %w", err)
		}
		log.Printf("Created default config file at '%s'", args.ConfigPath)
	}

	// Validate ResultMode.
	switch args.ResultMode {
	case model.OPEN_MODE_NONE, model.OPEN_MODE_WEB, model.OPEN_MODE_FILE:
		// Valid modes
	default:
		return fmt.Errorf("invalid ResultMode '%s', supported only: %s, %s, %s", args.ResultMode, model.OPEN_MODE_NONE, model.OPEN_MODE_WEB, model.OPEN_MODE_FILE)
	}

	// Prepare flags for writing to file and opening file with result.
	isWriteToFile := !args.DontBuildTextReport
	isOpenFileWithResult := args.ResultMode == model.OPEN_MODE_FILE

	// Parse configuration.
	cfg, err := config.ReadConfig(args.ConfigPath)
	if err != nil {
		return handleError(
			fmt.Errorf("configuration file '%s' is wrong: %w", args.ConfigPath, err),
			isWriteToFile,
			isOpenFileWithResult,
		)
	}

	// Ensure we're running in a terminal window before doing anything else.
	if cfg.EnsureTerminal && !args.NoTerminal {
		platform.EnsureTerminalWindow()
	}

	// Parse timezone or set system.
	timeZone, err := time.LoadLocation(cfg.TimeZoneLocation)
	if err != nil {
		return handleError(
			fmt.Errorf("unknown TimeZoneLocation: %s", cfg.TimeZoneLocation),
			isWriteToFile,
			isOpenFileWithResult,
		)
	}

	// Set language.
	if cfg.Language != "" {
		i18n.SetLocale(langToLocale[cfg.Language])
	}

	// Log settings.
	log.Println(i18n.T("Using configuration", "config", cfg))

	// Create data handler and parse files.
	dataHandler := &app.DataHandler{
		ConfigPath: args.ConfigPath,
		Config:     cfg,
		TimeZone:   timeZone,
	}
	transactions, fileInfos, parsingWarnings, cat, err := dataHandler.ParseAllFiles()
	if err != nil {
		return handleError(err, isWriteToFile, isOpenFileWithResult)
	}

	// Just show uncategorized transactions if in "CategorizeMode" and not WEB result mode.
	if cfg.CategorizeMode && args.ResultMode != model.OPEN_MODE_WEB {
		err = cat.PrintUncategorizedTransactions(transactions)
		if err != nil {
			return handleError(err, isWriteToFile, isOpenFileWithResult)
		}
		return nil
	}

	// Build DataMart and StatisticBuilderFactory.
	dataMart, err := currency.BuildDataMart(transactions, cfg)
	if err != nil {
		return handleError(err, isWriteToFile, isOpenFileWithResult)
	}
	statisticBuilderFactory, err := statistic.NewStatisticBuilderByCategories(dataMart.Accounts, cfg)
	if err != nil {
		return handleError(err, isWriteToFile, isOpenFileWithResult)
	}

	// Complete the DataHandler setup.
	dataHandler.DataMart = dataMart
	dataHandler.StatisticBuilderFactory = statisticBuilderFactory
	dataHandler.Categorization = cat
	dataHandler.FileInfos = fileInfos

	// Build journal entries.
	journalEntries, err := dataHandler.GetJournalEntries()
	if err != nil {
		return handleError(errors.New(i18n.T("can't build journal entries", "err", err)), isWriteToFile, isOpenFileWithResult)
	}

	// Produce Beancount file if not disabled.
	if !args.DontBuildBeanconFile {
		// Check that all transactions have Reciever/Payer account number.
		sourcesWithBrokenTransactions := make(map[string]struct{})
		for _, jEntry := range journalEntries {
			if jEntry.ToAccount == "" || jEntry.FromAccount == "" {
				sourcesWithBrokenTransactions[jEntry.Source.TypeName] = struct{}{}
			}
		}
		if len(sourcesWithBrokenTransactions) > 0 {
			sourceNames := make([]string, 0, len(sourcesWithBrokenTransactions))
			for sourceName := range sourcesWithBrokenTransactions {
				sourceNames = append(sourceNames, sourceName)
			}
			log.Println(i18n.T("can't build Beancount report, transactions from following sources don't have Reciever/Payer account number: sources", "sources", strings.Join(sourceNames, ", ")))
		} else {
			// Build Beancount file.
			transLen, err := beancount.BuildBeancountFile(journalEntries, dataMart.AllCurrencies, dataMart.Accounts, model.RESULT_BEANCOUNT_FILE_PATH)
			if err != nil {
				return handleError(errors.New(i18n.T("can't build Beancount report", "err", err)), isWriteToFile, isOpenFileWithResult)
			}
			log.Println(i18n.T("Built Beancount file f with n transactions", "file", model.RESULT_BEANCOUNT_FILE_PATH, "n", transLen))
		}
	}

	// Build statistic.
	monthlyStatistics, err := dataHandler.GetMonthlyStatistics()
	if err != nil {
		return handleError(
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

		cur := ""
		// For text report use first currency from ConvertToCurrencies or just first available currency.
		if len(cfg.ConvertToCurrencies) > 0 {
			cur = cfg.ConvertToCurrencies[0]
		} else {
			cur = journalEntries[0].AccountCurrency
			if cur == "" {
				cur = journalEntries[0].OriginCurrency
			}
		}
		for _, oneMonthStatistics := range monthlyStatistics {
			if err := statistic.DumpIntervalStatistics(oneMonthStatistics, &reportStringBuilder, cur, cfg.DetailedOutput); err != nil {
				return handleError(errors.New(i18n.T("can't dump interval statistics", "err", err)), isWriteToFile, isOpenFileWithResult)
			}
		}
		fmt.Fprintf(&reportStringBuilder, "\n%s", i18n.T("Total n months", "n", len(monthlyStatistics)))
		result := reportStringBuilder.String()

		// Always print result into logs and conditionally into the file which open through the OS.
		log.Println(result)
		if !args.DontBuildTextReport {
			platform.WriteAndOpenFile(model.RESULT_FILE_PATH, result, isOpenFileWithResult)
		}
	}

	// Start web server if needed.
	if args.ResultMode == model.OPEN_MODE_WEB {
		url := fmt.Sprintf("http://localhost:%d", dataHandler.Config.UIPort)
		go func() {
			time.Sleep(100 * time.Millisecond) // Give the server a moment to start.
			err := platform.OpenInOS(url)
			if err != nil {
				log.Println(i18n.T("Failed to open UI in web browser", "err", err))
			}
		}()

		log.Println(i18n.T("Starting local web server on urlport", "urlport", url))
		err := ui.ListenAndServe(dataHandler, static, templateFS, devMode)
		if err != nil {
			return handleError(
				errors.New(i18n.T("failed to start web server, probably app is already running", "err", err)),
				isWriteToFile,
				isOpenFileWithResult,
			)
		}
	}

	return nil
}

// handleError handles errors similar to fatalError but returns error instead of calling log.Fatal
func handleError(err error, inFile bool, openFile bool) error {
	errMsg := fmt.Sprintf("ERROR: %s", err)
	if inFile {
		platform.WriteAndOpenFile(model.RESULT_FILE_PATH, errMsg, openFile)
	}
	return errors.New(errMsg)
}
