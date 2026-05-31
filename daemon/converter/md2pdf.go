package converter

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuin/goldmark"
)

// MD2PDF converts markdown text to a PDF file.
// title is used for the HTML <title> and output filename.
// outputDir is where the generated PDF is saved.
// Uses goldmark for MD→HTML and wkhtmltopdf for HTML→PDF.
func MD2PDF(markdown, title, outputDir string) (string, error) {
	// 1. Render markdown to HTML via goldmark
	var htmlBuf bytes.Buffer
	if err := goldmark.Convert([]byte(markdown), &htmlBuf); err != nil {
		return "", fmt.Errorf("goldmark render: %w", err)
	}

	// 2. Wrap in a minimal HTML document with readable styling
	doc := wrapHTML(title, htmlBuf.String())

	// 3. Write temporary HTML file
	tmpHTML := filepath.Join(outputDir, "_temp.html")
	if err := os.WriteFile(tmpHTML, []byte(doc), 0o644); err != nil {
		return "", fmt.Errorf("write temp html: %w", err)
	}
	defer os.Remove(tmpHTML)

	// 4. Convert HTML to PDF via wkhtmltopdf
	slug := slugify(title)
	if slug == "" {
		slug = time.Now().UTC().Format("20060102-150405")
	}
	pdfPath := filepath.Join(outputDir, slug+".pdf")

	if err := exec.Command("wkhtmltopdf",
		"--quiet",
		"--enable-local-file-access",
		"--page-size", "A4",
		"--margin-top", "15mm",
		"--margin-bottom", "15mm",
		"--margin-left", "15mm",
		"--margin-right", "15mm",
		tmpHTML,
		pdfPath,
	).Run(); err != nil {
		return "", fmt.Errorf("wkhtmltopdf failed (is it installed? brew install wkhtmltopdf): %w", err)
	}

	if _, err := os.Stat(pdfPath); err != nil {
		return "", fmt.Errorf("pdf not generated: %w", err)
	}
	return pdfPath, nil
}

// wrapHTML embeds rendered markdown HTML into a minimal document.
func wrapHTML(title, body string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<style>
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    font-size: 12pt;
    line-height: 1.7;
    max-width: 800px;
    margin: 0 auto;
    padding: 20px;
    color: #1a1a1a;
  }
  h1 { font-size: 1.6em; border-bottom: 1px solid #ddd; padding-bottom: 0.3em; }
  h2 { font-size: 1.3em; margin-top: 1.5em; }
  h3 { font-size: 1.1em; }
  pre { background: #f5f5f5; padding: 12px; border-radius: 4px; overflow-x: auto; font-size: 0.9em; }
  code { background: #f5f5f5; padding: 2px 5px; border-radius: 3px; font-size: 0.9em; }
  pre code { padding: 0; }
  blockquote { border-left: 3px solid #ddd; margin-left: 0; padding-left: 15px; color: #555; }
  img { max-width: 100%%; }
  table { border-collapse: collapse; width: 100%%; }
  th, td { border: 1px solid #ddd; padding: 6px 12px; text-align: left; }
  a { color: #0366d6; }
</style>
</head>
<body>
%s
</body>
</html>`, escapeHTML(title), body)
}

// slugify creates a filesystem-safe name from a title.
func slugify(title string) string {
	s := strings.ToLower(title)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		if r == ' ' {
			return '-'
		}
		return -1
	}, s)
	s = strings.Trim(s, "-_")
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
