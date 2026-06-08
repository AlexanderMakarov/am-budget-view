package ui

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

	"github.com/AlexanderMakarov/am-budget-view/internal/app"
	"github.com/AlexanderMakarov/am-budget-view/internal/bankdownload"
	"github.com/AlexanderMakarov/am-budget-view/internal/config"
	"github.com/AlexanderMakarov/am-budget-view/internal/docs"
	"github.com/AlexanderMakarov/am-budget-view/internal/i18n"
	"github.com/AlexanderMakarov/am-budget-view/internal/model"
	"github.com/AlexanderMakarov/am-budget-view/internal/platform"
)

var templateFunctions template.FuncMap

// Pre-parsed shared templates (used only in production mode)
var sharedTemplates *template.Template

// embeddedStatic and embeddedTemplates hold the embedded FS passed during init.
var embeddedStatic embed.FS
var embeddedTemplates embed.FS
var isDevMode bool

// initTemplateFunctions sets up template functions when i18n is initialized.
func initTemplateFunctions() {
	templateFunctions = template.FuncMap{
		"localize": i18n.T,
		"formatDate": func(date time.Time) string {
			return i18n.T("date_format", "val", date)
		},
		"formatOptionalDate": func(date time.Time) string {
			if date.IsZero() {
				return "—"
			}
			return i18n.T("date_format", "val", date)
		},
		"formatRFC3339": func(ts string) string {
			if ts == "" {
				return "—"
			}
			parsed, err := time.Parse(time.RFC3339, ts)
			if err != nil {
				return ts
			}
			return i18n.T("date_format", "val", parsed)
		},
		"statusLabel": func(status string) string {
			labels := map[string]string{
				bankdownload.StatusOK:         "Up to date",
				bankdownload.StatusStale:      "Stale",
				bankdownload.StatusNoFiles:    "No files",
				bankdownload.StatusError:      "Error",
				bankdownload.StatusManualOnly: "Manual only",
			}
			if label, ok := labels[status]; ok {
				return i18n.T(label)
			}
			return status
		},
		"downloadMethodLabel": func(h bankdownload.SourceHealth) string {
			if h.SupportsInAppDownload {
				if h.DownloadConfigured {
					return i18n.T("In-app download")
				}
				return i18n.T("In-app download (not configured)")
			}
			if h.SupportsCliDownload {
				return i18n.T("CLI or manual")
			}
			return i18n.T("Manual")
		},
		"sourceID": func(id bankdownload.SourceID) string {
			return string(id)
		},
		"formatFreshnessDays": func(days int) string {
			if days < 0 {
				return "—"
			}
			if days == 0 {
				return i18n.T("0 days")
			}
			if days == 1 {
				return i18n.T("1 day")
			}
			return i18n.T("N days", "n", days)
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
	if !isDevMode {
		sharedTemplates = template.New("shared").Funcs(templateFunctions)

		// Read all shared template files from embedded FS
		entries, err := embeddedTemplates.ReadDir("templates/shared")
		if err != nil {
			return fmt.Errorf("failed to read shared templates directory: %w", err)
		}

		// Parse each shared template
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".html") {
				content, err := embeddedTemplates.ReadFile("templates/shared/" + entry.Name())
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

// ListenAndServe starts the web UI server.
func ListenAndServe(dataHandler *app.DataHandler, staticFS embed.FS, templateFS embed.FS, docsFS embed.FS, devMode bool) error {
	embeddedStatic = staticFS
	embeddedTemplates = templateFS
	isDevMode = devMode
	docs.Init(docsFS, devMode)

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
	http.HandleFunc("/open-file", handleOpenFile())
	http.HandleFunc("/refresh-files", handleRefreshFiles(dataHandler))
	http.HandleFunc("/bank-downloads/settings", handleBankDownloadSettings(dataHandler))
	http.HandleFunc("/bank-downloads/config", handleBankDownloadsConfig(dataHandler))
	http.HandleFunc("/bank-downloads/run", handleBankDownloadsRun(dataHandler))
	http.HandleFunc("/docs/bank/", handleBankDoc())

	// Serve static files based on DEV_MODE
	if isDevMode {
		// In development mode, serve from filesystem
		http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	} else {
		// In production mode, serve from embedded FS
		http.Handle("/static/", http.FileServer(http.FS(embeddedStatic)))
	}

	// Wrap the entire http.ServeMux with a logging handler
	loggedMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a custom ResponseWriter to capture the status code.
		lw := &logWriter{ResponseWriter: w}
		http.DefaultServeMux.ServeHTTP(lw, r)
		duration := time.Since(start)
		log.Printf("%s %s %d %dms", r.Method, r.URL.Path, lw.statusCode, duration.Milliseconds())
	})

	if isDevMode {
		log.Println("Running in development mode - serving static files directly from filesystem")
	}
	return http.ListenAndServe(fmt.Sprintf(":%d", dataHandler.Config.UIPort), loggedMux)
}

func handleIndex(dataHandler *app.DataHandler) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Handle locale selection.
		locale := r.URL.Query().Get("locale")
		if locale != "" {
			err := i18n.SetLocale(locale)
			if err != nil {
				log.Printf("Failed to set locale to %s: %v", locale, err)
			}
			log.Printf("Set locale to %s", i18n.GetLocale())
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
			Locale:     i18n.GetLocale(),
		}

		err = parseAndExecuteTemplate("templates/index.html", w, data)
		if err != nil {
			logAndReturnError(w, err)
			return
		}
	}
}

func handleTransactions(dataHandler *app.DataHandler) http.HandlerFunc {
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
		var entries []model.JournalEntry
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
			model.JournalEntry
			FromAccountInfo *model.AccountStatistics
			ToAccountInfo   *model.AccountStatistics
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

		// Get sorted groups for the template
		sortedGroups := getSortedGroups(dataHandler.Config.Groups)

		// JSON encode the groups
		jsonGroups, err := json.Marshal(sortedGroups)
		if err != nil {
			logAndReturnError(w, err)
			return
		}

		data := struct {
			Month    string
			Group    string
			Type     string
			Currency string
			Entries  []TemplateEntry
			Accounts template.JS
			Groups   template.JS
		}{
			Month:    month,
			Group:    group,
			Type:     txType,
			Currency: currency,
			Entries:  templateEntries,
			Accounts: template.JS(string(jsonAccounts)),
			Groups:   template.JS(string(jsonGroups)),
		}

		err = parseAndExecuteTemplate("templates/transactions.html", w, data)
		if err != nil {
			logAndReturnError(w, err)
			return
		}
	}
}

func handleCategorization(dataHandler *app.DataHandler) http.HandlerFunc {
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
					logAndReturnError(w, fmt.Errorf("for action 'upsertGroup' value in 'groupName' should be provided"))
					return
				}
				if group, ok := dataHandler.Config.Groups[request.GroupName]; ok {
					group.Substrings = request.Substrings
					group.FromAccounts = request.FromAccounts
					group.ToAccounts = request.ToAccounts
				} else {
					group = &config.GroupConfig{
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
			Transactions []model.Transaction
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

func handleGroups(dataHandler *app.DataHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := struct {
			Groups map[string]*config.GroupConfig
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

func handleFiles(dataHandler *app.DataHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workingDir, err := os.Getwd()
		if err != nil {
			workingDir = i18n.T("Unable to determine working directory")
		}

		language := dataHandler.Config.Language
		if language == "" {
			language = "en"
		}

		data := struct {
			WorkingDir string
			FileGroups bankdownload.FileGroupsView
			Language   string
		}{
			WorkingDir: workingDir,
			FileGroups: bankdownload.BuildFileGroups(dataHandler.FileInfos, dataHandler.Config),
			Language:   language,
		}

		err = parseAndExecuteTemplate("templates/files.html", w, data)
		if err != nil {
			logAndReturnError(w, err)
			return
		}
	}
}

func handleBankDownloadSettings(dataHandler *app.DataHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		data := struct {
			MyAmeria       config.MyAmeriaDownloadConfig
			AmeriaBusiness config.AmeriaBusinessDownloadConfig
		}{
			MyAmeria:       dataHandler.Config.BankDownloads.MyAmeria,
			AmeriaBusiness: dataHandler.Config.BankDownloads.AmeriaBusiness,
		}

		if err := parseAndExecuteTemplate("templates/bank_download_settings.html", w, data); err != nil {
			logAndReturnError(w, err)
		}
	}
}

func handleOpenFile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filePath := r.URL.Query().Get("path")
		if filePath == "" {
			http.Error(w, "No file path provided", http.StatusBadRequest)
			return
		}

		if err := platform.OpenInOS(filePath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func handleBankDoc() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sourceID := strings.TrimPrefix(r.URL.Path, "/docs/bank/")
		if sourceID == "" || strings.Contains(sourceID, "/") {
			http.NotFound(w, r)
			return
		}
		if !docs.ValidSourceIDs[sourceID] {
			http.NotFound(w, r)
			return
		}

		markdown, err := docs.LoadBankDoc(i18n.GetLocale(), sourceID)
		if err != nil {
			log.Printf("Bank doc not found: %s: %v", sourceID, err)
			http.NotFound(w, r)
			return
		}

		htmlContent, err := docs.RenderHTML(markdown)
		if err != nil {
			logAndReturnError(w, err)
			return
		}

		title := sourceID
		if idx := strings.Index(markdown, "\n"); idx > 0 {
			title = strings.TrimPrefix(strings.TrimSpace(markdown[:idx]), "# ")
		}

		data := struct {
			Title   string
			Content template.HTML
			Locale  string
		}{
			Title:   title,
			Content: template.HTML(htmlContent),
			Locale:  i18n.GetLocale(),
		}

		if err := parseAndExecuteTemplate("templates/bank_doc.html", w, data); err != nil {
			logAndReturnError(w, err)
		}
	}
}

func handleRefreshFiles(dataHandler *app.DataHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		log.Println("Refreshing files - re-parsing all transaction files...")

		err := dataHandler.RebuildFromFiles()
		if err != nil {
			log.Printf("Error refreshing files: %v", err)
			logAndReturnError(w, err)
			return
		}

		log.Println("Files refreshed successfully")
		w.WriteHeader(http.StatusOK)
	}
}

type bankDownloadJSONResponse struct {
	OK      bool     `json:"ok"`
	Error   string   `json:"error,omitempty"`
	Hint    string   `json:"hint,omitempty"`
	Message string   `json:"message,omitempty"`
	Files   []string `json:"files,omitempty"`
}

func writeBankDownloadJSON(w http.ResponseWriter, status int, resp bankDownloadJSONResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

type bankDownloadsConfigRequest struct {
	BankDownloads bankDownloadsJSONPatch `json:"bankDownloads"`
}

type bankDownloadsJSONPatch struct {
	StaleThresholdDays *int                         `json:"staleThresholdDays,omitempty"`
	MyAmeria           *myAmeriaDownloadJSONPatch   `json:"myAmeria,omitempty"`
	AmeriaBusiness     *ameriaBusinessDownloadJSONPatch `json:"ameriaBusiness,omitempty"`
}

type myAmeriaDownloadJSONPatch struct {
	Enabled            *bool  `json:"enabled,omitempty"`
	ClientId           string `json:"clientId,omitempty"`
	AuthToken          string `json:"authToken,omitempty"`
	SinceDate          string `json:"sinceDate,omitempty"`
	LastDownloadAt     string `json:"lastDownloadAt,omitempty"`
	LastDownloadStatus string `json:"lastDownloadStatus,omitempty"`
	LastDownloadError  string `json:"lastDownloadError,omitempty"`
}

type ameriaBusinessDownloadJSONPatch struct {
	Enabled            *bool  `json:"enabled,omitempty"`
	Cookie             string `json:"cookie,omitempty"`
	SinceDate          string `json:"sinceDate,omitempty"`
	OutputFolder         string `json:"outputFolder,omitempty"`
	LastDownloadAt     string `json:"lastDownloadAt,omitempty"`
	LastDownloadStatus string `json:"lastDownloadStatus,omitempty"`
	LastDownloadError  string `json:"lastDownloadError,omitempty"`
}

func mergeMyAmeriaJSONPatch(current config.MyAmeriaDownloadConfig, patch *myAmeriaDownloadJSONPatch) config.MyAmeriaDownloadConfig {
	if patch == nil {
		return current
	}
	out := current
	if patch.ClientId != "" {
		out.ClientId = patch.ClientId
	}
	if patch.AuthToken != "" {
		out.AuthToken = bankdownload.NormalizeMyAmeriaAuthToken(patch.AuthToken)
	}
	if patch.SinceDate != "" {
		out.SinceDate = patch.SinceDate
	}
	if patch.LastDownloadAt != "" {
		out.LastDownloadAt = patch.LastDownloadAt
	}
	if patch.LastDownloadStatus != "" {
		out.LastDownloadStatus = patch.LastDownloadStatus
	}
	if patch.LastDownloadError != "" {
		out.LastDownloadError = patch.LastDownloadError
	}
	if patch.Enabled != nil {
		out.Enabled = *patch.Enabled
	}
	return out
}

func mergeAmeriaBusinessJSONPatch(current config.AmeriaBusinessDownloadConfig, patch *ameriaBusinessDownloadJSONPatch) config.AmeriaBusinessDownloadConfig {
	if patch == nil {
		return current
	}
	out := current
	if patch.Cookie != "" {
		out.Cookie = patch.Cookie
	}
	if patch.SinceDate != "" {
		out.SinceDate = patch.SinceDate
	}
	if patch.OutputFolder != "" {
		out.OutputFolder = patch.OutputFolder
	}
	if patch.LastDownloadAt != "" {
		out.LastDownloadAt = patch.LastDownloadAt
	}
	if patch.LastDownloadStatus != "" {
		out.LastDownloadStatus = patch.LastDownloadStatus
	}
	if patch.LastDownloadError != "" {
		out.LastDownloadError = patch.LastDownloadError
	}
	if patch.Enabled != nil {
		out.Enabled = *patch.Enabled
	}
	return out
}

func handleBankDownloadsConfig(dataHandler *app.DataHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req bankDownloadsConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeBankDownloadJSON(w, http.StatusBadRequest, bankDownloadJSONResponse{
				OK:    false,
				Error: "Invalid JSON body",
				Hint:  err.Error(),
			})
			return
		}

		current := dataHandler.Config.BankDownloads
		if req.BankDownloads.MyAmeria != nil {
			dataHandler.Config.BankDownloads.MyAmeria = mergeMyAmeriaJSONPatch(current.MyAmeria, req.BankDownloads.MyAmeria)
		}
		if req.BankDownloads.AmeriaBusiness != nil {
			dataHandler.Config.BankDownloads.AmeriaBusiness = mergeAmeriaBusinessJSONPatch(current.AmeriaBusiness, req.BankDownloads.AmeriaBusiness)
		}

		var patch config.BankDownloads
		if req.BankDownloads.StaleThresholdDays != nil {
			patch.StaleThresholdDays = *req.BankDownloads.StaleThresholdDays
		}

		if err := dataHandler.UpdateBankDownloads(patch); err != nil {
			ufe := bankdownload.MapError(err)
			if ufe.Message == "" {
				ufe.Message = err.Error()
			}
			writeBankDownloadJSON(w, http.StatusBadRequest, bankDownloadJSONResponse{
				OK:    false,
				Error: ufe.Message,
				Hint:  ufe.Hint,
			})
			return
		}

		writeBankDownloadJSON(w, http.StatusOK, bankDownloadJSONResponse{OK: true})
	}
}

func handleBankDownloadsRun(dataHandler *app.DataHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			SourceID       string                      `json:"sourceId"`
			MyAmeria       *myAmeriaDownloadJSONPatch  `json:"myAmeria,omitempty"`
			AmeriaBusiness *ameriaBusinessDownloadJSONPatch `json:"ameriaBusiness,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeBankDownloadJSON(w, http.StatusBadRequest, bankDownloadJSONResponse{
				OK:    false,
				Error: "Invalid JSON body",
				Hint:  err.Error(),
			})
			return
		}
		if req.SourceID == "" {
			writeBankDownloadJSON(w, http.StatusBadRequest, bankDownloadJSONResponse{
				OK:    false,
				Error: "sourceId is required",
			})
			return
		}

		var opts *app.BankDownloadRunOptions
		if req.MyAmeria != nil || req.AmeriaBusiness != nil {
			opts = &app.BankDownloadRunOptions{}
			if req.MyAmeria != nil {
				cfg := mergeMyAmeriaJSONPatch(dataHandler.Config.BankDownloads.MyAmeria, req.MyAmeria)
				opts.MyAmeria = &cfg
			}
			if req.AmeriaBusiness != nil {
				cfg := mergeAmeriaBusinessJSONPatch(dataHandler.Config.BankDownloads.AmeriaBusiness, req.AmeriaBusiness)
				opts.AmeriaBusiness = &cfg
			}
		}

		log.Printf("bank download run: source=%s", req.SourceID)
		paths, err := dataHandler.DownloadBank(req.SourceID, opts)
		if err != nil {
			log.Printf("bank download run failed: source=%s error=%v", req.SourceID, err)
			ufe := bankdownload.MapError(err)
			if ufe.Message == "" {
				ufe.Message = err.Error()
			}
			writeBankDownloadJSON(w, http.StatusBadRequest, bankDownloadJSONResponse{
				OK:    false,
				Error: ufe.Message,
				Hint:  ufe.Hint,
			})
			return
		}

		resp := bankDownloadJSONResponse{OK: true, Files: paths}
		if n := len(paths); n > 0 {
			resp.Message = fmt.Sprintf("Downloaded %d file(s)", n)
			log.Printf("bank download run ok: source=%s files=%v", req.SourceID, paths)
		} else {
			log.Printf("bank download run ok: source=%s (no files written)", req.SourceID)
		}
		writeBankDownloadJSON(w, http.StatusOK, resp)
	}
}

// mustEncodeJSON encodes JSON and panics on error.
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

	if isDevMode {
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
		content, err := embeddedTemplates.ReadFile(templatePath)
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

func getSortedGroups(groups map[string]*config.GroupConfig) map[string]*config.GroupConfig {
	// Get sorted group names
	var groupNames []string
	for name := range groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)

	// Create sorted groups map
	sortedGroups := make(map[string]*config.GroupConfig)
	for _, name := range groupNames {
		sortedGroups[name] = groups[name]
	}
	return sortedGroups
}
