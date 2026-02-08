package app

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/AlexanderMakarov/am-budget-view/internal/categorization"
	"github.com/AlexanderMakarov/am-budget-view/internal/config"
	"github.com/AlexanderMakarov/am-budget-view/internal/currency"
	"github.com/AlexanderMakarov/am-budget-view/internal/i18n"
	"github.com/AlexanderMakarov/am-budget-view/internal/model"
	"github.com/AlexanderMakarov/am-budget-view/internal/parser"
	"github.com/AlexanderMakarov/am-budget-view/internal/statistic"
)

// DataHandler is a handler for data.
// Contains methods to recalculate, cache, persist data.
type DataHandler struct {
	// ConfigPath is a path to the configuration file.
	ConfigPath string
	// Config is a configuration.
	Config *config.Config
	// TimeZone is a time zone.
	TimeZone *time.Location
	// DataMart is a set of data to build journal entries.
	DataMart *currency.DataMart
	// StatisticBuilderFactory is a factory to create statistic builders by categories.
	StatisticBuilderFactory statistic.StatisticBuilderFactory
	// Categorization is a cached struct to categorize transactions.
	Categorization *categorization.Categorization
	// journalEntries is a list of cached journal entries.
	journalEntries []model.JournalEntry
	// uncategorizedTransactions is a list of cached uncategorized transactions.
	uncategorizedTransactions []model.Transaction
	// monthlyStatistics is a list of cached monthly statistics.
	monthlyStatistics []map[string]*model.IntervalStatistic
	// FileInfos is a list of cached file information.
	FileInfos []model.FileInfo
}

func NewDataHandler(configPath string, initialConfig *config.Config, timeZone *time.Location, dataMart *currency.DataMart, groupExtractorFactory statistic.StatisticBuilderFactory, initialCategorization *categorization.Categorization, fileInfos []model.FileInfo) *DataHandler {
	return &DataHandler{
		ConfigPath:              configPath,
		Config:                  initialConfig,
		TimeZone:                timeZone,
		DataMart:                dataMart,
		StatisticBuilderFactory: groupExtractorFactory,
		Categorization:          initialCategorization,
		FileInfos:               fileInfos,
	}
}

func (dh *DataHandler) rebuildJournalEntriesAndUncategorizedTransactions() error {
	var err error
	if dh.Categorization == nil {
		dh.Categorization, err = categorization.NewCategorization(dh.Config)
		if err != nil {
			return err
		}
	}
	dh.journalEntries, dh.uncategorizedTransactions, err = currency.BuildJournalEntries(dh.DataMart, dh.Categorization)
	if err != nil {
		return err
	}
	return nil
}

// GetJournalEntries returns journal entries.
// If journalEntries are already built, returns them from cache.
// Otherwise builds journal entries and returns them.
// Note that it also builds accounts, currencies and uncategorized transactions.
func (dh *DataHandler) GetJournalEntries() ([]model.JournalEntry, error) {
	if dh.journalEntries == nil {
		err := dh.rebuildJournalEntriesAndUncategorizedTransactions()
		if err != nil {
			return nil, err
		}
	}
	return dh.journalEntries, nil
}

func (dh *DataHandler) GetUncategorizedTransactions() ([]model.Transaction, error) {
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
	dh.monthlyStatistics, err = statistic.BuildMonthlyStatistics(
		journalEntries,
		dh.StatisticBuilderFactory,
		dh.Config.MonthStartDayNumber,
		dh.TimeZone,
	)
	if err != nil {
		return err
	}
	return nil
}

func (dh *DataHandler) GetMonthlyStatistics() ([]map[string]*model.IntervalStatistic, error) {
	if dh.monthlyStatistics == nil {
		err := dh.rebuildMonthlyStatistics()
		if err != nil {
			return nil, err
		}
	}
	return dh.monthlyStatistics, nil
}

func (dh *DataHandler) UpdateGroups(groups map[string]*config.GroupConfig) error {
	dh.Config.Groups = groups
	err := dh.Config.WriteToFile(dh.ConfigPath)
	if err != nil {
		return err
	}
	// Clear caches.
	dh.Categorization = nil
	dh.journalEntries = nil
	dh.uncategorizedTransactions = nil
	dh.monthlyStatistics = nil
	return nil
}

// ParseAllFiles parses all transaction files from the current configuration.
// Doesn't update DataHandler fields.
// Returns transactions, file infos, parsing warnings, categorization, and error.
func (dh *DataHandler) ParseAllFiles() ([]model.Transaction, []model.FileInfo, []string, *categorization.Categorization, error) {
	var allFileInfos []model.FileInfo
	transactions := make([]model.Transaction, 0)
	parsingWarnings := []string{}

	// Parse files to unified Transaction-s.
	// Ineco XML
	inecoXmlTransactions, fileInfos, err := parser.ParseTransactionsOfOneType(
		dh.Config.InecobankStatementXmlFilesGlob,
		"Inecobank XML statement",
		parser.InecoXmlParser{},
		&parsingWarnings,
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	transactions = append(transactions, inecoXmlTransactions...)
	allFileInfos = append(allFileInfos, fileInfos...)

	// Ineco XLSX
	inecoXlsxTransactions, fileInfos, err := parser.ParseTransactionsOfOneType(
		dh.Config.InecobankStatementXlsxFilesGlob,
		"Inecobank XLSX statement",
		parser.InecoExcelFileParser{},
		&parsingWarnings,
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	transactions = append(transactions, inecoXlsxTransactions...)
	allFileInfos = append(allFileInfos, fileInfos...)

	// MyAmeria Excel account statements and history.
	myAmeriaStatementsXlsTransactions, fileInfos, err := parser.ParseTransactionsOfOneType(
		dh.Config.MyAmeriaAccountStatementXlsFilesGlob,
		"MyAmeria XLS statement",
		parser.MyAmeriaExcelStmtFileParser{},
		&parsingWarnings,
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	transactions = append(transactions, myAmeriaStatementsXlsTransactions...)
	allFileInfos = append(allFileInfos, fileInfos...)
	myAmeriaHistoryXlsTransactions, fileInfos, err := parser.ParseTransactionsOfOneType(
		dh.Config.MyAmeriaHistoryXlsFilesGlob,
		"MyAmeria History XLS",
		parser.MyAmeriaExcelFileParser{
			MyAccounts: dh.Config.MyAmeriaMyAccounts,
		},
		&parsingWarnings,
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	transactions = append(transactions, myAmeriaHistoryXlsTransactions...)
	allFileInfos = append(allFileInfos, fileInfos...)

	// Ameria CSV
	ameriaCsvTransactions, fileInfos, err := parser.ParseTransactionsOfOneType(
		dh.Config.AmeriaCsvFilesGlob,
		"AmeriaBank CSV statement",
		parser.AmeriaCsvFileParser{},
		&parsingWarnings,
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	transactions = append(transactions, ameriaCsvTransactions...)
	allFileInfos = append(allFileInfos, fileInfos...)

	// Arshinbank XLSX
	ardshinbankXlsxTransactions, fileInfos, err := parser.ParseTransactionsOfOneType(
		dh.Config.ArdshinbankXlsxFilesGlob,
		"Ardshinbank XLSX statement",
		parser.ArdshinXlsxFileParser{},
		&parsingWarnings,
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	transactions = append(transactions, ardshinbankXlsxTransactions...)
	allFileInfos = append(allFileInfos, fileInfos...)

	// Acba Regular Account XLS
	acbaRegularAccountXlsTransactions, fileInfos, err := parser.ParseTransactionsOfOneType(
		dh.Config.AcbaRegularAccountXlsFilesGlob,
		"Acba Regular Account XLS statement",
		parser.AcbaRegularAccountExcelFileParser{},
		&parsingWarnings,
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	transactions = append(transactions, acbaRegularAccountXlsTransactions...)
	allFileInfos = append(allFileInfos, fileInfos...)

	// Acba Card XLS
	acbaCardXlsTransactions, fileInfos, err := parser.ParseTransactionsOfOneType(
		dh.Config.AcbaCardXlsFilesGlob,
		"Acba Card XLS statement",
		parser.AcbaCardExcelFileParser{},
		&parsingWarnings,
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	transactions = append(transactions, acbaCardXlsTransactions...)
	allFileInfos = append(allFileInfos, fileInfos...)

	// Generic CSV
	genericCsvTransactions, fileInfos, err := parser.ParseTransactionsOfOneType(
		dh.Config.GenericCsvFilesGlob,
		"Generic CSV with transactions",
		parser.GenericCsvFileParser{},
		&parsingWarnings,
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	transactions = append(transactions, genericCsvTransactions...)
	allFileInfos = append(allFileInfos, fileInfos...)

	if len(transactions) < 1 {
		return nil, nil, nil, nil, errors.New(
			i18n.T("can't find transactions, parsing warnings w", "w", parsingWarnings),
		)
	}
	log.Println(i18n.T("Total found n transactions", "n", len(transactions)))

	// Create initial Categorization.
	cat, err := categorization.NewCategorization(dh.Config)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return transactions, allFileInfos, parsingWarnings, cat, nil
}

// RebuildFromFiles rebuilds the DataHandler by re-reading the config file and re-parsing all transaction files.
// This method is useful for the UI to refresh all data when files or config have been updated.
func (dh *DataHandler) RebuildFromFiles() error {
	// Re-read configuration file to catch any user changes.
	cfg, err := config.ReadConfig(dh.ConfigPath)
	if err != nil {
		return fmt.Errorf("configuration file '%s' is wrong: %w", dh.ConfigPath, err)
	}

	// Update stored config
	dh.Config = cfg

	// Re-parse all files using the updated config
	transactions, fileInfos, parsingWarnings, cat, err := dh.ParseAllFiles()
	if err != nil {
		return err
	}

	// Log parsing warnings if any
	if len(parsingWarnings) > 0 {
		for _, warning := range parsingWarnings {
			log.Println("Parsing warning:", warning)
		}
	}

	// Rebuild DataMart with new transactions
	newDataMart, err := currency.BuildDataMart(transactions, cfg)
	if err != nil {
		return err
	}

	// Update DataHandler with new data
	dh.DataMart = newDataMart
	dh.Categorization = cat
	dh.FileInfos = fileInfos

	// Clear cached data to force recalculation
	dh.journalEntries = nil
	dh.uncategorizedTransactions = nil
	dh.monthlyStatistics = nil

	// Rebuild GroupExtractorFactory with new accounts
	groupExtractorFactory, err := statistic.NewStatisticBuilderByCategories(dh.DataMart.Accounts, dh.Config)
	if err != nil {
		return err
	}
	dh.StatisticBuilderFactory = groupExtractorFactory

	return nil
}
