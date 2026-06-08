package bankdownload

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlexanderMakarov/am-budget-view/internal/config"
)

const (
	ameriaBusinessAPIBase = "https://gateway-businessmyameria.ameriabank.am/api/v1"
	ameriaBusinessOrigin  = "https://business.myameria.am"

	accountTypeSettlement = "SettlementAccount"
	accountTypeCard       = "CardAccount"

	ameriaBusinessTimeout = 60 * time.Second
	sinceDateLayout       = "02-01-2006"
	apiDateLayout         = "2006-01-02"

	// ameriaBusinessMaxStatementBytes caps the statement-CSV response read to avoid
	// unbounded memory use on an unexpectedly large or malicious response (16 MiB).
	ameriaBusinessMaxStatementBytes = 16 << 20
)

type ameriaBusinessAccount struct {
	ID            int
	Number        string
	Currency      string
	Name          string
	SubType       string
	Balance       float64
	AccountStatus string
	Description   string
}

type ameriaBusinessClient struct {
	httpClient *http.Client
	apiBase    string
	origin     string
}

func newAmeriaBusinessClient(httpClient *http.Client) *ameriaBusinessClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: ameriaBusinessTimeout}
	}
	return &ameriaBusinessClient{
		httpClient: httpClient,
		apiBase:    ameriaBusinessAPIBase,
		origin:     ameriaBusinessOrigin,
	}
}

func ameriaBusinessHeaders(cookie, origin string) http.Header {
	h := http.Header{}
	h.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
	h.Set("Accept", "application/json, text/plain, */*")
	h.Set("Accept-Language", "en-US")
	h.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	h.Set("Referer", origin+"/")
	h.Set("Origin", origin)
	h.Set("Model", "firefox")
	h.Set("DeviceOS", "Linux")
	h.Set("DeviceId", "MyAmeriaBusiness Web")
	h.Set("DeviceOSVersion", "147.0.0")
	h.Set("Platform", "Web")
	h.Set("Connection", "keep-alive")
	h.Set("Cookie", latin1Safe(cookie))
	h.Set("Sec-Fetch-Dest", "empty")
	h.Set("Sec-Fetch-Mode", "cors")
	h.Set("Sec-Fetch-Site", "cross-site")
	return h
}

func latin1Safe(s string) string {
	if s == "" {
		return s
	}
	var buf bytes.Buffer
	for _, r := range s {
		if r <= 0xFF {
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

// normalizeAmeriaBusinessCookie trims and removes line breaks from pasted Cookie headers.
func normalizeAmeriaBusinessCookie(cookie string) string {
	cookie = strings.TrimSpace(cookie)
	cookie = strings.ReplaceAll(cookie, "\r\n", "")
	cookie = strings.ReplaceAll(cookie, "\n", "")
	cookie = strings.ReplaceAll(cookie, "\r", "")
	return strings.TrimSpace(cookie)
}

func ameriaBusinessCookieNames(cookie string) []string {
	names := make([]string, 0, 4)
	for _, part := range strings.Split(cookie, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, _, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func hasCookieName(names []string, want string) bool {
	want = strings.ToLower(want)
	for _, name := range names {
		if strings.EqualFold(name, want) {
			return true
		}
	}
	return false
}

// ErrIncompleteCookie indicates the pasted Cookie header lacks required session keys.
var ErrIncompleteCookie = errors.New("ameria business cookie incomplete")

func validateAmeriaBusinessCookie(cookie string) error {
	names := ameriaBusinessCookieNames(cookie)
	if len(names) == 0 {
		return fmt.Errorf("%w: no cookie pairs found", ErrIncompleteCookie)
	}
	if !hasCookieName(names, "RefreshToken") {
		return fmt.Errorf(
			"%w: missing RefreshToken (got %d chars, keys=%v)",
			ErrIncompleteCookie,
			len(cookie),
			names,
		)
	}
	return nil
}

func (c *ameriaBusinessClient) refreshCookie(cookie string) (string, error) {
	url := c.apiBase + "/Authentication/Refresh"
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader("{}"))
	if err != nil {
		return "", err
	}
	req.Header = ameriaBusinessHeaders(cookie, c.origin)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Storage-Access", "none")

	resp, err := c.httpClient.Do(req)
	var body []byte
	if resp != nil {
		defer resp.Body.Close()
		body, _ = io.ReadAll(io.LimitReader(resp.Body, 500))
	}
	logHTTPExchange("AmeriaBusiness/Refresh", req.Method, req.URL.String(), req.Header, resp, body, err)
	if err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("%w: %s", ErrRefreshUnauthorized, bodyPreview(body, httpLogBodyPreviewMax))
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: HTTP %d: %s", ErrRefreshFailed, resp.StatusCode, bodyPreview(body, httpLogBodyPreviewMax))
	}

	cookies := resp.Cookies()
	if len(cookies) == 0 {
		return "", fmt.Errorf("%w: no Set-Cookie in response", ErrRefreshFailed)
	}
	parts := make([]string, 0, len(cookies))
	for _, ck := range cookies {
		parts = append(parts, ck.Name+"="+ck.Value)
	}
	return strings.Join(parts, "; "), nil
}

func (c *ameriaBusinessClient) fetchAccounts(cookie, accountType string) ([]ameriaBusinessAccount, error) {
	url := fmt.Sprintf(
		"%s/Accounts?pageIndex=1&pageSize=100&accountType=%s&accountStatus=Open",
		c.apiBase,
		accountType,
	)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header = ameriaBusinessHeaders(cookie, c.origin)
	tag := fmt.Sprintf("AmeriaBusiness/Accounts(%s)", accountType)

	resp, err := c.httpClient.Do(req)
	var body []byte
	if resp != nil {
		defer resp.Body.Close()
		body, _ = io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	}
	logHTTPExchange(tag, req.Method, req.URL.String(), req.Header, resp, body, err)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, &unauthorizedError{body: string(body), op: "Accounts"}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &ameriaBusinessHTTPError{
			statusCode: resp.StatusCode,
			op:         "Accounts",
			body:       strings.TrimSpace(string(body)),
		}
	}

	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("AmeriaBank Business Accounts response is not valid JSON: %w", err)
	}

	accounts := make([]ameriaBusinessAccount, 0, len(items))
	for _, item := range items {
		acc, err := parseAmeriaBusinessAccount(item, accountType)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, acc)
	}
	return accounts, nil
}

func parseAmeriaBusinessAccount(item map[string]any, defaultType string) (ameriaBusinessAccount, error) {
	id, err := jsonNumberInt(item["id"])
	if err != nil {
		return ameriaBusinessAccount{}, fmt.Errorf("account id: %w", err)
	}
	number, _ := item["number"].(string)
	currency, _ := item["currency"].(string)
	name := stringField(item, "name")
	if name == "" {
		name = stringField(item, "description")
	}
	subType := stringField(item, "subType")
	if subType == "" {
		subType = defaultType
	}
	balance, _ := jsonNumberFloat(item["balance"])
	return ameriaBusinessAccount{
		ID:            id,
		Number:        number,
		Currency:      currency,
		Name:          name,
		SubType:       subType,
		Balance:       balance,
		AccountStatus: stringField(item, "accountStatus"),
		Description:   stringField(item, "description"),
	}, nil
}

func stringField(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func jsonNumberInt(v any) (int, error) {
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case json.Number:
		i, err := n.Int64()
		return int(i), err
	default:
		return 0, fmt.Errorf("expected number, got %T", v)
	}
}

func jsonNumberFloat(v any) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case json.Number:
		return n.Float64()
	default:
		return 0, nil
	}
}

func (c *ameriaBusinessClient) downloadStatementCSV(
	cookie string,
	accountID int,
	startDate, endDate, path string,
) error {
	url := fmt.Sprintf(
		"%s/Accounts/%d/Statements/Export?exportFormat=Csv&startDate=%s&endDate=%s&withAmd=true",
		c.apiBase,
		accountID,
		startDate,
		endDate,
	)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header = ameriaBusinessHeaders(cookie, c.origin)
	req.Header.Set("Accept", "text/csv, application/csv, application/json, */*")
	tag := fmt.Sprintf("AmeriaBusiness/Export(accountId=%d)", accountID)

	resp, err := c.httpClient.Do(req)
	var raw []byte
	if resp != nil {
		defer resp.Body.Close()
		raw, err = io.ReadAll(io.LimitReader(resp.Body, ameriaBusinessMaxStatementBytes))
	}
	logHTTPExchange(tag, req.Method, req.URL.String(), req.Header, resp, raw, err)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return &unauthorizedError{body: string(raw[:min(len(raw), 500)]), op: "Export"}
	}
	if resp.StatusCode != http.StatusOK {
		snippet := strings.TrimSpace(string(raw[:min(len(raw), 500)]))
		return &ameriaBusinessHTTPError{statusCode: resp.StatusCode, op: "Export", body: snippet}
	}

	text := string(raw)
	if strings.HasPrefix(strings.TrimSpace(text), "{") {
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err == nil {
			if data, ok := obj["data"].(string); ok {
				text = data
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func (c *ameriaBusinessClient) with401Refresh(cookie string, fn func(string) error) (string, error) {
	err := fn(cookie)
	if err == nil {
		return cookie, nil
	}
	if !errors.Is(err, ErrUnauthorized) {
		var unauth *unauthorizedError
		if !errors.As(err, &unauth) {
			return cookie, err
		}
	}
	newCookie, refreshErr := c.refreshCookie(cookie)
	if refreshErr != nil {
		return cookie, refreshErr
	}
	return newCookie, fn(newCookie)
}

func sanitizeFilenamePart(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.TrimSpace(s)
	if s == "" {
		return "account"
	}
	return s
}

func resolveOutputDir(wd, outputFolder string) string {
	outputFolder = strings.TrimSpace(outputFolder)
	if outputFolder == "" || outputFolder == "." {
		return wd
	}

	var target string
	if filepath.IsAbs(outputFolder) {
		target = outputFolder
	} else {
		target = filepath.Join(wd, outputFolder)
	}

	targetAbs, err := filepath.Abs(target)
	if err != nil {
		log.Printf("AmeriaBusiness: outputFolder %q is invalid (%v); using working directory", outputFolder, err)
		return wd
	}
	wdAbs, err := filepath.Abs(wd)
	if err != nil {
		return targetAbs
	}
	rel, err := filepath.Rel(wdAbs, targetAbs)
	if err != nil || strings.HasPrefix(rel, "..") {
		log.Printf(
			"AmeriaBusiness: outputFolder %q resolves outside working directory %q; using working directory instead",
			outputFolder,
			wdAbs,
		)
		return wdAbs
	}
	return targetAbs
}

func sinceDateToAPI(sinceDate string) (string, error) {
	t, err := time.Parse(sinceDateLayout, sinceDate)
	if err != nil {
		return "", fmt.Errorf("invalid sinceDate %q: must be DD-MM-YYYY: %w", sinceDate, err)
	}
	return t.Format(apiDateLayout), nil
}

// DownloadAmeriaBusiness downloads CSV statements for all open settlement and card accounts.
func DownloadAmeriaBusiness(cfg config.AmeriaBusinessDownloadConfig, wd string) ([]string, error) {
	return downloadAmeriaBusiness(cfg, wd, newAmeriaBusinessClient(nil))
}

func downloadAmeriaBusiness(cfg config.AmeriaBusinessDownloadConfig, wd string, client *ameriaBusinessClient) ([]string, error) {
	cfg.Cookie = normalizeAmeriaBusinessCookie(cfg.Cookie)
	if cfg.Cookie == "" {
		return nil, fmt.Errorf("ameria business cookie is required — paste Cookie header from DevTools in Download settings")
	}
	if err := validateAmeriaBusinessCookie(cfg.Cookie); err != nil {
		return nil, err
	}

	startDate, err := sinceDateToAPI(cfg.SinceDate)
	if err != nil {
		return nil, err
	}
	endDate := time.Now().Format(apiDateLayout)
	log.Printf("AmeriaBusiness: download start sinceDate=%q apiRange=%s..%s outputFolder=%q cookieBytes=%d keys=%v",
		cfg.SinceDate, startDate, endDate, cfg.OutputFolder, len(cfg.Cookie), ameriaBusinessCookieNames(cfg.Cookie))

	cookie := cfg.Cookie

	var settlement, card []ameriaBusinessAccount

	cookie, err = client.with401Refresh(cookie, func(c string) error {
		var fetchErr error
		settlement, fetchErr = client.fetchAccounts(c, accountTypeSettlement)
		return fetchErr
	})
	if err != nil {
		return nil, err
	}

	cookie, err = client.with401Refresh(cookie, func(c string) error {
		var fetchErr error
		card, fetchErr = client.fetchAccounts(c, accountTypeCard)
		return fetchErr
	})
	if err != nil {
		return nil, err
	}

	accounts := append(settlement, card...)
	log.Printf("AmeriaBusiness: found %d settlement + %d card accounts", len(settlement), len(card))
	baseDir := resolveOutputDir(wd, cfg.OutputFolder)
	sinceSafe := strings.ReplaceAll(cfg.SinceDate, "/", "-")

	var paths []string
	for _, acc := range accounts {
		outPath := filepath.Join(
			baseDir,
			fmt.Sprintf("AccountStatement %s %s since %s.csv", acc.Number, sanitizeFilenamePart(acc.Name), sinceSafe),
		)
		acc := acc
		cookie, err = client.with401Refresh(cookie, func(c string) error {
			return client.downloadStatementCSV(c, acc.ID, startDate, endDate, outPath)
		})
		if err != nil {
			return paths, err
		}
		paths = append(paths, outPath)
		log.Printf("AmeriaBusiness: wrote %q", outPath)
	}
	log.Printf("AmeriaBusiness: download complete — %d file(s) in %q", len(paths), baseDir)
	return paths, nil
}
