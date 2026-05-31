// Hugging Face Paper Producer - downloads original arXiv PDFs for HF paper URLs.
// Placed in ~/.send2nlm/producer/ and run as a compiled external plugin.
//
// Prerequisites: opencli must be installed with the hf site adapter available.

package producer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"send2nlm/sdk"
)

// HFPaperProducer downloads the original PDF for Hugging Face paper URLs.
type HFPaperProducer struct{}

func (p *HFPaperProducer) Name() string { return "hf-paper" }

var (
	hfPaperPattern = regexp.MustCompile(`^https?://(?:www\.)?huggingface\.co/papers/([^/?#]+)`)
	hfArxivIDRe    = regexp.MustCompile(`^([0-9]{4}\.[0-9]{4,5})(?:v[0-9]+)?$`)
)

func (p *HFPaperProducer) Match(rawURL string) bool {
	return hfPaperPattern.MatchString(rawURL)
}

func (p *HFPaperProducer) Produce(ctx context.Context, rawURL string) (string, error) {
	id, err := hfPaperIDFromURL(rawURL)
	if err != nil {
		return "", err
	}

	outputDir := filepath.Join(os.TempDir(), "send2nlm", "hf-paper-"+time.Now().UTC().Format("20060102-150405"))
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("hf paper mkdir: %w", err)
	}

	paper, err := fetchHFPaper(ctx, id)
	if err != nil {
		return "", fmt.Errorf("hf paper metadata: %w", err)
	}

	pdfURL := "https://arxiv.org/pdf/" + id
	pdfPath := filepath.Join(outputDir, safeHFPaperFilename(paper.Title, id))
	if err := downloadHFPaperPDF(ctx, pdfURL, pdfPath); err != nil {
		return "", fmt.Errorf("download original PDF: %w", err)
	}
	return pdfPath, nil
}

type hfPaper struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

func hfPaperIDFromURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}
	normalized := parsed.Scheme + "://" + parsed.Host + parsed.EscapedPath()

	m := hfPaperPattern.FindStringSubmatch(normalized)
	if len(m) != 2 {
		return "", fmt.Errorf("unsupported Hugging Face paper URL: %s", rawURL)
	}
	return normalizeHFArxivID(m[1])
}

func normalizeHFArxivID(value string) (string, error) {
	id := strings.TrimSpace(value)
	id = strings.TrimSuffix(id, ".pdf")
	id = strings.TrimPrefix(id, "arXiv:")
	if m := hfArxivIDRe.FindStringSubmatch(id); len(m) == 2 {
		return m[1], nil
	}
	return "", fmt.Errorf("unsupported Hugging Face paper id: %s", value)
}

func fetchHFPaper(ctx context.Context, id string) (hfPaper, error) {
	var papers []hfPaper
	if err := runHFOpenCLIJSON(ctx, &papers, "hf", "paper", id, "-f", "json"); err != nil {
		return hfPaper{}, err
	}
	if len(papers) == 0 {
		return hfPaper{}, fmt.Errorf("no paper returned by opencli")
	}
	return papers[0], nil
}

func runHFOpenCLIJSON(ctx context.Context, target any, args ...string) error {
	cmd := exec.CommandContext(ctx, "opencli", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, msg)
	}

	if err := json.Unmarshal(extractHFOpenCLIJSON(stdout.Bytes()), target); err != nil {
		return fmt.Errorf("parse opencli json: %w", err)
	}
	return nil
}

func extractHFOpenCLIJSON(raw []byte) []byte {
	s := strings.TrimSpace(string(raw))
	start := strings.IndexAny(s, "[{")
	if start < 0 {
		return raw
	}
	s = s[start:]

	if strings.HasPrefix(s, "[") {
		if end := strings.LastIndex(s, "]"); end >= 0 {
			return []byte(s[:end+1])
		}
	}
	if strings.HasPrefix(s, "{") {
		if end := strings.LastIndex(s, "}"); end >= 0 {
			return []byte(s[:end+1])
		}
	}
	return []byte(s)
}

func downloadHFPaperPDF(ctx context.Context, rawURL, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "send2nlm/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s returned %s", rawURL, resp.Status)
	}

	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return err
	}
	return nil
}

func safeHFPaperFilename(title, fallbackID string) string {
	name := strings.TrimSpace(title)
	if name == "" {
		name = fallbackID
	}
	name = regexp.MustCompile(`[^A-Za-z0-9._-]+`).ReplaceAllString(name, "_")
	name = strings.Trim(name, "._-")
	if name == "" {
		name = fallbackID
	}
	if len(name) > 120 {
		name = name[:120]
	}
	return name + ".pdf"
}

// Producer is the exported variable required by the external plugin wrapper.
var Producer sdk.Producer = &HFPaperProducer{}
