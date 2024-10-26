package main

import (
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"time"
)

//go:embed static
var static embed.FS

//go:embed templates
var templateFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "templates/*.html"))

func ListenAndServe(statistics []map[string]*IntervalStatistic) {
	http.HandleFunc("/", handleIndex(statistics))

	// Serve static files
	http.Handle("/static/", http.FileServer(http.FS(static)))

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

		data := struct {
			Title      string
			Currencies []string
			Statistics template.JS
		}{
			Title:      "Interval Statistics",
			Currencies: currencies,
			Statistics: template.JS(jsonData),
		}

		err = templates.ExecuteTemplate(w, "index.html", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
