package producer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"send2nlm/converter"
	"send2nlm/sdk"
)

// DefaultProducer is the universal fallback producer.
// It fetches a web page as Markdown plus images via opencli and renders the
// saved Markdown file as a PDF via pandoc.
// This is the only producer compiled into the daemon binary.
type DefaultProducer struct{}

// Ensure DefaultProducer implements sdk.Producer.
var _ sdk.Producer = (*DefaultProducer)(nil)

func (p *DefaultProducer) Name() string { return "default" }

func (p *DefaultProducer) Match(url string) bool { return true }

func (p *DefaultProducer) Produce(ctx context.Context, url string) (string, error) {
	// 1. Create output directory. opencli stores downloaded article images here.
	outputDir := defaultProducerOutputDir()
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("default producer mkdir: %w", err)
	}

	// 2. Fetch rendered page content as Markdown and image assets through opencli.
	article, err := fetchArticle(ctx, url, outputDir)
	if err != nil {
		return "", fmt.Errorf("default producer opencli fetch: %w", err)
	}

	// 3. Extract title.
	title := strings.TrimSpace(article.Title)
	if title == "" {
		markdown, err := os.ReadFile(article.Saved)
		if err != nil {
			return "", fmt.Errorf("default producer read markdown: %w", err)
		}
		title = extractTitle(string(markdown), url)
	}

	// 4. Convert Markdown file to PDF.
	pdfPath, err := converter.MDFile2PDF(article.Saved, title, outputDir)
	if err != nil {
		return "", fmt.Errorf("default producer md→pdf: %w", err)
	}

	return pdfPath, nil
}

func defaultProducerOutputDir() string {
	baseDir := os.Getenv("SEND2NLM_PRODUCER_OUTPUT_DIR")
	if baseDir == "" {
		baseDir = filepath.Join(os.TempDir(), "send2nlm")
	}
	return filepath.Join(baseDir, time.Now().UTC().Format("20060102-150405"))
}

type opencliArticle struct {
	Title  string `json:"title"`
	Status string `json:"status"`
	Saved  string `json:"saved"`
}

// fetchArticle asks opencli to render the page and save article Markdown/assets.
func fetchArticle(ctx context.Context, url, outputDir string) (opencliArticle, error) {
	cmd := exec.CommandContext(ctx,
		"opencli",
		"web",
		"read",
		"--url", url,
		"--output", outputDir,
		"--download-images", "true",
		"--wait", "3",
		"-f", "json",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return opencliArticle{}, err
		}
		return opencliArticle{}, fmt.Errorf("%w: %s", err, msg)
	}

	var articles []opencliArticle
	if err := json.Unmarshal(stdout.Bytes(), &articles); err != nil {
		return opencliArticle{}, fmt.Errorf("parse opencli json: %w", err)
	}
	if len(articles) == 0 {
		return opencliArticle{}, fmt.Errorf("no article returned by opencli")
	}

	article := articles[0]
	if article.Status != "" && article.Status != "success" {
		return opencliArticle{}, fmt.Errorf("opencli returned status %q", article.Status)
	}
	if article.Saved == "" {
		return opencliArticle{}, fmt.Errorf("opencli did not return saved markdown path")
	}
	if !filepath.IsAbs(article.Saved) {
		article.Saved = filepath.Join(outputDir, article.Saved)
	}
	if _, err := os.Stat(article.Saved); err != nil {
		return opencliArticle{}, fmt.Errorf("opencli markdown not found: %w", err)
	}

	return article, nil
}

var markdownTitleRe = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)

// extractTitle pulls the first Markdown H1, falling back to the URL path.
func extractTitle(markdown, url string) string {
	if m := markdownTitleRe.FindStringSubmatch(markdown); len(m) >= 2 {
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
