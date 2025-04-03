package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed static
var static embed.FS

//go:embed templates
var templateFS embed.FS

var templateFunctions template.FuncMap

// Pre-parsed shared templates (used only in production mode)
var sharedTemplates *template.Template

// initTemplateFunctions sets up template functions when i18n is initialized.
func initTemplateFunctions() {
	templateFunctions = template.FuncMap{
		"localize": i18n.T,
		"formatDate": func(date time.Time) string {
			return i18n.T("date_format", "val", date)
		},
		"toJSON": func(v interface{}) string {
			data, err := json.Marshal(v)
			if err != nil {
				return "{}"
			}
			return string(data)
		},
	}
}

// initSharedTemplates initializes shared templates in production mode.
func initSharedTemplates() error {
	if !devMode {
		sharedTemplates = template.New("shared").Funcs(templateFunctions)

		// Read all shared template files from embedded FS
		entries, err := templateFS.ReadDir("templates/shared")
		if err != nil {
			return fmt.Errorf("failed to read shared templates directory: %w", err)
		}

		// Parse each shared template
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".html") {
				content, err := templateFS.ReadFile("templates/shared/" + entry.Name())
				if err != nil {
					return fmt.Errorf("failed to read shared template %s: %w", entry.Name(), err)
				}

				// Use "shared/" prefix for template name
				templateName := "shared/" + filepath.Base(entry.Name())
				_, err = sharedTemplates.New(templateName).Parse(string(content))
				if err != nil {
					return fmt.Errorf("failed to parse shared template %s: %w", entry.Name(), err)
				}
			}
		}
	}
	return nil
}

func ListenAndServe(dataHandler *DataHandler) error {
	// Initialize template functions
	initTemplateFunctions()

	// Initialize shared templates
	if err := initSharedTemplates(); err != nil {
		return fmt.Errorf("failed to initialize shared templates: %w", err)
	}

	// Set up HTTP handlers
	http.HandleFunc("/", handleIndex(dataHandler))
	http.HandleFunc("/transactions", handleTransactions(dataHandler))
	http.HandleFunc("/categorization", handleCategorization(dataHandler))
	http.HandleFunc("/groups", handleGroups(dataHandler))
	http.HandleFunc("/files", handleFiles(dataHandler))
	http.HandleFunc("/open-file", handleOpenFile(dataHandler))

	// Serve static files based on DEV_MODE
	if devMode {
		// In development mode, serve from filesystem
		http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	} else {
		// In production mode, serve from embedded FS
		http.Handle("/static/", http.FileServer(http.FS(static)))
	}

	// Wrap the entire http.ServeMux with a logging handler
	loggedMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a custom ResponseWriter to capture the status code
		lw := &logWriter{ResponseWriter: w}
		http.DefaultServeMux.ServeHTTP(lw, r)
		duration := time.Since(start)
		log.Printf("%s %s %d %dms", r.Method, r.URL.Path, lw.statusCode, duration.Milliseconds())
	})

	if devMode {
		log.Println("Running in development mode - serving static files from filesystem")
	}
	return http.ListenAndServe(":8080", loggedMux)
}

func handleIndex(dataHandler *DataHandler) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Handle locale selection.
		locale := r.URL.Query().Get("locale")
		if locale != "" {
			err := i18n.SetLocale(locale)
			if err != nil {
				log.Printf("Failed to set locale to %s: %v", locale, err)
			}
			log.Printf("Set locale to %s", i18n.locale)
		}

		// Prepare JSON with statistics.
		statistics, err := dataHandler.GetMonthlyStatistics()
		if err != nil {
			logAndReturnError(w, err)
			return
		}
		jsonData, err := json.Marshal(statistics)
		if err != nil {
			logAndReturnError(w, err)
			return
		}

		currencies := make([]string, 0)
		for _, stat := range statistics[0] {
			currencies = append(currencies, stat.Currency)
		}
		sort.Strings(currencies)

		data := struct {
			Currencies []string
			Statistics template.JS
			Locale     string
		}{
			Currencies: currencies,
			Statistics: template.JS(jsonData),
			Locale:     i18n.locale,
		}

		err = parseAndExecuteTemplate("templates/index.html", w, data)
		if err != nil {
			logAndReturnError(w, err)
			return
		}
	}
}

func handleTransactions(dataHandler *DataHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		month := r.URL.Query().Get("month")
		group := r.URL.Query().Get("group")
		txType := r.URL.Query().Get("type")
		currency := r.URL.Query().Get("currency")

		statistics, err := dataHandler.GetMonthlyStatistics()
		if err != nil {
			logAndReturnError(w, err)
			return
		}

		// Find the statistics for the selected month
		var entries []JournalEntry
		for _, stat := range statistics {
			currStat := stat[currency]
			if currStat == nil {
				continue
			}

			// Break this into separate steps for debugging
			formattedDate := i18n.T("date_format", "val", currStat.Start)[:7] // YYYY-MM
			if formattedDate == month {
				if txType == "income" {
					if groupData, ok := currStat.Income[group]; ok {
						entries = groupData.JournalEntries
					}
				} else {
					if groupData, ok := currStat.Expense[group]; ok {
						entries = groupData.JournalEntries
					}
				}
				break
			}
		}

		// JSON encode the accounts
		jsonAccounts, err := json.Marshal(dataHandler.DataMart.Accounts)
		if err != nil {
			logAndReturnError(w, err)
			return
		}

		// Prepare data for the template
		type TemplateEntry struct {
			JournalEntry
			FromAccountInfo *AccountStatistics
			ToAccountInfo   *AccountStatistics
			IsCounted       bool
			Group           string
		}

		var templateEntries []TemplateEntry

		for _, entry := range entries {
			fromAccount := dataHandler.DataMart.Accounts[entry.FromAccount]
			toAccount := dataHandler.DataMart.Accounts[entry.ToAccount]
			isCounted := fromAccount != nil &&
				toAccount != nil &&
				fromAccount.IsTransactionAccount &&
				toAccount.IsTransactionAccount

			templateEntries = append(templateEntries, TemplateEntry{
				JournalEntry:    entry,
				FromAccountInfo: fromAccount,
				ToAccountInfo:   toAccount,
				IsCounted:       isCounted,
				Group:           group,
			})
		}

		data := struct {
			Month    string
			Group    string
			Type     string
			Currency string
			Entries  []TemplateEntry
			Accounts template.JS
		}{
			Month:    month,
			Group:    group,
			Type:     txType,
			Currency: currency,
			Entries:  templateEntries,
			Accounts: template.JS(string(jsonAccounts)),
		}

		err = parseAndExecuteTemplate("templates/transactions.html", w, data)
		if err != nil {
			logAndReturnError(w, err)
			return
		}
	}
}

func handleCategorization(dataHandler *DataHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var request struct {
				Action       string   `json:"action"`
				GroupName    string   `json:"groupName"`
				NewGroupName string   `json:"newGroupName,omitempty"`
				Substrings   []string `json:"substrings,omitempty"`
				FromAccounts []string `json:"fromAccounts,omitempty"`
				ToAccounts   []string `json:"toAccounts,omitempty"`
			}

			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			switch request.Action {
			case "upsertGroup":
				if request.GroupName == "" {
					logAndReturnError(w, fmt.Errorf("for 'upsertGroup' action 'groupName' is required"))
					return
				}
				if group, ok := dataHandler.Config.Groups[request.GroupName]; ok {
					group.Substrings = request.Substrings
					group.FromAccounts = request.FromAccounts
					group.ToAccounts = request.ToAccounts
				} else {
					group = &GroupConfig{
						Substrings:   request.Substrings,
						FromAccounts: request.FromAccounts,
						ToAccounts:   request.ToAccounts,
					}
					dataHandler.Config.Groups[request.GroupName] = group
				}

			case "deleteGroup":
				if request.GroupName == "" {
					logAndReturnError(w, fmt.Errorf("for 'deleteGroup' action 'groupName' is required"))
					return
				}
				delete(dataHandler.Config.Groups, request.GroupName)

			case "renameGroup":
				if request.NewGroupName == "" {
					logAndReturnError(w, fmt.Errorf("newGroupName is required"))
					return
				}
				if _, exists := dataHandler.Config.Groups[request.NewGroupName]; exists {
					http.Error(w, "Group with this name already exists", http.StatusBadRequest)
					return
				}
				group := dataHandler.Config.Groups[request.GroupName]
				delete(dataHandler.Config.Groups, request.GroupName)
				dataHandler.Config.Groups[request.NewGroupName] = group
			}

			// After any modification update groups in memory and on disk.
			if err := dataHandler.UpdateGroups(dataHandler.Config.Groups); err != nil {
				logAndReturnError(w, err)
				return
			}

			// Return updated list of uncategorized transactions
		}

		// Show the categorization page.
		transactions, err := dataHandler.GetUncategorizedTransactions()
		if err != nil {
			logAndReturnError(w, err)
			return
		}
		data := struct {
			Transactions []Transaction
			Groups       template.JS
			Accounts     template.JS
		}{
			Transactions: transactions,
			Groups:       template.JS(mustEncodeJSON(getSortedGroups(dataHandler.Config.Groups))),
			Accounts:     template.JS(mustEncodeJSON(dataHandler.DataMart.Accounts)),
		}
		err = parseAndExecuteTemplate("templates/categorization.html", w, data)
		if err != nil {
			logAndReturnError(w, err)
			return
		}
	}
}

func handleGroups(dataHandler *DataHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := struct {
			Groups map[string]*GroupConfig
		}{
			Groups: getSortedGroups(dataHandler.Config.Groups),
		}

		err := parseAndExecuteTemplate("templates/groups.html", w, data)
		if err != nil {
			logAndReturnError(w, err)
			return
		}
	}
}

func handleFiles(dataHandler *DataHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workingDir, err := os.Getwd()
		if err != nil {
			workingDir = i18n.T("Unable to determine working directory")
		}

		data := struct {
			WorkingDir string
			Files      []FileInfo
		}{
			WorkingDir: workingDir,
			Files:      dataHandler.FileInfos,
		}

		err = parseAndExecuteTemplate("templates/files.html", w, data)
		if err != nil {
			logAndReturnError(w, err)
			return
		}
	}
}

func handleOpenFile(dataHandler *DataHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filePath := r.URL.Query().Get("path")
		if filePath == "" {
			http.Error(w, "No file path provided", http.StatusBadRequest)
			return
		}

		if err := openFileInOS(filePath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// Helper function to encode JSON and panic on error (since this is server startup)
func mustEncodeJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func logAndReturnError(w http.ResponseWriter, err error) {
	log.Printf("Error: %v", err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// logWriter is a custom ResponseWriter that captures the status code
type logWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lw *logWriter) WriteHeader(code int) {
	lw.statusCode = code
	lw.ResponseWriter.WriteHeader(code)
}

func (lw *logWriter) Write(b []byte) (int, error) {
	if lw.statusCode == 0 {
		lw.statusCode = 200
	}
	return lw.ResponseWriter.Write(b)
}

func parseAndExecuteTemplate(templatePath string, w http.ResponseWriter, data interface{}) error {
	// Set cache-control headers before any writes to response
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	var tmpl *template.Template

	if devMode {
		// In dev mode, parse both shared and main templates from filesystem
		tmpl = template.New(filepath.Base(templatePath)).Funcs(templateFunctions)

		// Get list of all shared template files
		sharedFiles, err := filepath.Glob("templates/shared/*.html")
		if err != nil {
			return fmt.Errorf("failed to list shared templates: %w", err)
		}

		// Parse each shared template with "shared/" prefix
		for _, sharedFile := range sharedFiles {
			baseName := filepath.Base(sharedFile)
			templateName := "shared/" + baseName
			content, err := os.ReadFile(sharedFile)
			if err != nil {
				return fmt.Errorf("failed to read shared template %s: %w", sharedFile, err)
			}
			_, err = tmpl.New(templateName).Parse(string(content))
			if err != nil {
				return fmt.Errorf("failed to parse shared template %s: %w", sharedFile, err)
			}
		}

		// Parse the main template
		content, err := os.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", templatePath, err)
		}
		_, err = tmpl.Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", templatePath, err)
		}
	} else {
		// In production mode, clone pre-parsed shared templates
		var err error
		tmpl, err = sharedTemplates.Clone()
		if err != nil {
			return fmt.Errorf("failed to clone shared templates: %w", err)
		}

		// Read and parse the main template
		content, err := templateFS.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", templatePath, err)
		}

		// Parse the main template content
		_, err = tmpl.New(filepath.Base(templatePath)).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", templatePath, err)
		}
	}

	// Execute using the base name of the template
	return tmpl.ExecuteTemplate(w, filepath.Base(templatePath), data)
}

func getSortedGroups(groups map[string]*GroupConfig) map[string]*GroupConfig {
	// Get sorted group names
	var groupNames []string
	for name := range groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)

	// Create sorted groups map
	sortedGroups := make(map[string]*GroupConfig)
	for _, name := range groupNames {
		sortedGroups[name] = groups[name]
	}
	return sortedGroups
}
