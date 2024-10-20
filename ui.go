package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
)

func ListenAndServe(statistics []*IntervalStatistic) {
	http.HandleFunc("/", handleIndex(statistics))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	log.Println("Server starting on http://localhost:8080")
	err := http.ListenAndServe(":8080", nil) // Changed to ListenAndServe without TLS
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func handleIndex(statistics []*IntervalStatistic) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("templates/index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// JSON encode the statistics
		jsonData, err := json.Marshal(statistics)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := struct {
			Title      string
			Statistics template.JS
		}{
			Title:      "Interval Statistics",
			Statistics: template.JS(jsonData),
		}

		err = tmpl.Execute(w, data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, a := range slice {
		if a == item {
			return true
		}
	}
	return false
}
