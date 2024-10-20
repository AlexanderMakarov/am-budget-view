package main

import (
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

		// Prepare data for the template
		var labels []string
		var incomeData []float64
		var expenseData []float64
		var incomeGroups []string
		var expenseGroups []string

		for _, stat := range statistics {
			labels = append(labels, stat.Start.Format("2006-01"))
			
			var totalIncome float64
			var totalExpense float64
			
			for group, data := range stat.Income {
				totalIncome += float64(data.Total.int) / 100
				if !contains(incomeGroups, group) {
					incomeGroups = append(incomeGroups, group)
				}
			}
			
			for group, data := range stat.Expense {
				totalExpense += float64(data.Total.int) / 100
				if !contains(expenseGroups, group) {
					expenseGroups = append(expenseGroups, group)
				}
			}
			
			incomeData = append(incomeData, totalIncome)
			expenseData = append(expenseData, totalExpense)
		}

		data := struct {
			Title         string
			Labels        []string
			IncomeData    []float64
			ExpenseData   []float64
			IncomeGroups  []string
			ExpenseGroups []string
			Statistics    []*IntervalStatistic
		}{
			Title:         "Expenses vs Income",
			Labels:        labels,
			IncomeData:    incomeData,
			ExpenseData:   expenseData,
			IncomeGroups:  incomeGroups,
			ExpenseGroups: expenseGroups,
			Statistics:    statistics,
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
