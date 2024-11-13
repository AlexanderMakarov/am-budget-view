package main

import (
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"time"
)

//go:embed static
var static embed.FS

//go:embed templates
var templateFS embed.FS

var templateFunctions template.FuncMap

// initTemplateFunctions sets up template functions when i18n is initialized.
func initTemplateFunctions() {
	templateFunctions = template.FuncMap{
		"localize": i18n.T,
	}
}

func ListenAndServe(statistics []map[string]*IntervalStatistic, accounts map[string]*AccountFromTransactions) error {
	initTemplateFunctions()

	http.HandleFunc("/", handleIndex(statistics))
	http.HandleFunc("/transactions", handleTransactions(statistics, accounts))

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

func handleIndex(statistics []map[string]*IntervalStatistic) func(w http.ResponseWriter, r *http.Request) {
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
		jsonData, err := json.Marshal(statistics)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func handleTransactions(statistics []map[string]*IntervalStatistic, accounts map[string]*AccountFromTransactions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		month := r.URL.Query().Get("month")
		group := r.URL.Query().Get("group")
		txType := r.URL.Query().Get("type")
		currency := r.URL.Query().Get("currency")

		// Find the statistics for the selected month
		var entries []JournalEntry
		for _, stat := range statistics {
			currStat := stat[currency]
			if currStat == nil {
				continue
			}

			if currStat.Start.Format("2006-01") == month {
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
		jsonAccounts, err := json.Marshal(accounts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Prepare data for the template
		var templateEntries []struct {
			JournalEntry
			FromAccountInfo *AccountFromTransactions
			ToAccountInfo   *AccountFromTransactions
			IsCounted       bool
		}

		for _, entry := range entries {
			fromAccount := accounts[entry.FromAccount]
			toAccount := accounts[entry.ToAccount]
			// Check if both accounts exist and are transaction accounts
			isCounted := fromAccount != nil &&
				toAccount != nil &&
				fromAccount.IsTransactionAccount &&
				toAccount.IsTransactionAccount

			templateEntries = append(templateEntries, struct {
				JournalEntry
				FromAccountInfo *AccountFromTransactions
				ToAccountInfo   *AccountFromTransactions
				IsCounted       bool
			}{
				JournalEntry:    entry,
				FromAccountInfo: fromAccount,
				ToAccountInfo:   toAccount,
				IsCounted:       isCounted,
			})
		}

		data := struct {
			Month    string
			Group    string
			Type     string
			Currency string
			Entries  []struct {
				JournalEntry
				FromAccountInfo *AccountFromTransactions
				ToAccountInfo   *AccountFromTransactions
				IsCounted       bool
			}
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
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
	var tmpl *template.Template
	var err error
	if devMode {
		// In dev mode, load from filesystem with base name
		tmpl, err = template.New(templatePath).Funcs(templateFunctions).ParseFiles(templatePath)
	} else {
		// For embedded files, we need to read the content first
		content, err := templateFS.ReadFile(templatePath)
		if err != nil {
			return err
		}
		// Create template with the base name and parse the content
		baseName := filepath.Base(templatePath)
		tmpl, err = template.New(baseName).Funcs(templateFunctions).Parse(string(content))
	}
	if err != nil {
		return err
	}

	// Execute using the base name of the template
	return tmpl.ExecuteTemplate(w, filepath.Base(templatePath), data)
}
