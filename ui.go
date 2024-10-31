package main

import (
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"time"
)

//go:embed static
var static embed.FS

//go:embed templates
var templateFS embed.FS

var devMode bool = os.Getenv("DEV_MODE") != ""
var templates = template.Must(template.ParseFS(templateFS, "templates/*.html"))

func ListenAndServe(statistics []map[string]*IntervalStatistic, accounts map[string]*AccountFromTransactions) {
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

	log.Println("Server starting on http://localhost:8080")
	if devMode {
		log.Println("Running in development mode - serving static files from filesystem")
	}
	err := http.ListenAndServe(":8080", loggedMux)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func handleIndex(statistics []map[string]*IntervalStatistic) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// JSON encode the statistics
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
			Title      string
			Currencies []string
			Statistics template.JS
		}{
			Title:      "Interval Statistics",
			Currencies: currencies,
			Statistics: template.JS(jsonData),
		}

		if devMode {
			tmpl, err := template.ParseFiles("templates/index.html")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			err = tmpl.Execute(w, data)
		} else {
			err = templates.ExecuteTemplate(w, "index.html", data)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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

		data := struct {
			Month    string
			Group    string
			Type     string
			Currency string
			Entries  []JournalEntry
			Accounts template.JS
		}{
			Month:    month,
			Group:    group,
			Type:     txType,
			Currency: currency,
			Entries:  entries,
			Accounts: template.JS(string(jsonAccounts)),
		}

		if devMode {
			tmpl, err := template.ParseFiles("templates/transactions.html")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			err = tmpl.Execute(w, data)
		} else {
			err = templates.ExecuteTemplate(w, "transactions.html", data)
		}
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
