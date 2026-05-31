package producer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"send2nlm/converter"
	"send2nlm/sdk"
)

// DefaultProducer is the universal fallback producer.
// It fetches a web page via HTTP, converts the HTML to Markdown,
// and renders the Markdown as a PDF via goldmark + wkhtmltopdf.
// This is the only producer compiled into the daemon binary.
type DefaultProducer struct{}

// Ensure DefaultProducer implements sdk.Producer.
var _ sdk.Producer = (*DefaultProducer)(nil)

func (p *DefaultProducer) Name() string { return "default" }

func (p *DefaultProducer) Match(url string) bool { return true }

func (p *DefaultProducer) Produce(ctx context.Context, url string) (string, error) {
	// 1. HTTP GET the page
	html, err := fetchPage(ctx, url)
	if err != nil {
		return "", fmt.Errorf("default producer fetch: %w", err)
	}

	// 2. Extract title
	title := extractTitle(html, url)

	// 3. Convert HTML to Markdown
	markdown, err := converter.HTMLToMarkdown(html)
	if err != nil {
		return "", fmt.Errorf("default producer html→md: %w", err)
	}
	if markdown == "" {
		return "", fmt.Errorf("default producer: no content extracted from %s", url)
	}

	// 4. Create output directory
	outputDir := filepath.Join(os.TempDir(), "send2nlm", time.Now().UTC().Format("20060102-150405"))
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("default producer mkdir: %w", err)
	}

	// 5. Convert Markdown to PDF
	pdfPath, err := converter.MD2PDF(markdown, title, outputDir)
	if err != nil {
		return "", fmt.Errorf("default producer md→pdf: %w", err)
	}

	return pdfPath, nil
}

// fetchPage performs an HTTP GET and returns the response body as a string.
func fetchPage(ctx context.Context, url string) (string, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Send2NLM/1.0; +https://github.com/send2nlm)")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Limit response to 10 MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", err
	}

	return string(body), nil
}

var titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// extractTitle pulls the page title from HTML, falling back to the URL path.
func extractTitle(html, url string) string {
	if m := titleRe.FindStringSubmatch(html); len(m) >= 2 {
		t := strings.TrimSpace(m[1])
		if t != "" {
			return t
		}
	}

	// Fallback: use the last meaningful path segment
	u := url
	if idx := strings.Index(u, "?"); idx >= 0 {
		u = u[:idx]
	}
	if idx := strings.Index(u, "#"); idx >= 0 {
		u = u[:idx]
	}
	parts := strings.Split(strings.TrimRight(u, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return "Untitled"
}
