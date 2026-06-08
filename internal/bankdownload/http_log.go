package bankdownload

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

const httpLogBodyPreviewMax = 160

// logHTTPExchange logs one HTTP round-trip as a single line (secrets redacted).
func logHTTPExchange(tag, method, url string, reqHeaders http.Header, resp *http.Response, body []byte, reqErr error) {
	var b strings.Builder
	b.WriteString(tag)
	b.WriteByte(' ')
	b.WriteString(method)
	b.WriteByte(' ')
	b.WriteString(url)

	if reqErr != nil {
		fmt.Fprintf(&b, " error=%v", reqErr)
		log.Print(b.String())
		return
	}
	if resp == nil {
		b.WriteString(" error=(no response)")
		log.Print(b.String())
		return
	}

	fmt.Fprintf(&b, " status=%d bytes=%d", resp.StatusCode, len(body))
	if hdr := formatHeaders(reqHeaders); hdr != "" {
		b.WriteString(" req{")
		b.WriteString(hdr)
		b.WriteByte('}')
	}
	if preview := bodyPreview(body, httpLogBodyPreviewMax); preview != "" {
		fmt.Fprintf(&b, " body=%s", preview)
	}
	log.Print(b.String())
}

func formatHeaders(h http.Header) string {
	if h == nil {
		return ""
	}
	parts := make([]string, 0, len(h))
	for _, key := range sortedHeaderKeys(h) {
		for _, value := range h.Values(key) {
			parts = append(parts, key+"="+redactHeaderValue(key, value))
		}
	}
	return strings.Join(parts, ", ")
}

func sortedHeaderKeys(h http.Header) []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if strings.ToLower(keys[i]) > strings.ToLower(keys[j]) {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

func redactHeaderValue(key, value string) string {
	switch strings.ToLower(key) {
	case "authorization", "cookie", "set-cookie":
		if value == "" {
			return "(empty)"
		}
		if strings.ToLower(key) == "set-cookie" {
			name := value
			if idx := strings.Index(value, "="); idx > 0 {
				name = value[:idx]
			}
			return name + "=(redacted)"
		}
		return fmt.Sprintf("(%d chars, redacted)", len(value))
	default:
		return value
	}
}

func bodyPreview(body []byte, maxLen int) string {
	preview := strings.TrimSpace(string(body))
	if preview == "" {
		return "(empty)"
	}
	preview = strings.ReplaceAll(preview, "\n", " ")
	preview = strings.ReplaceAll(preview, "\r", " ")
	if len(preview) > maxLen {
		return preview[:maxLen] + "…"
	}
	return preview
}
