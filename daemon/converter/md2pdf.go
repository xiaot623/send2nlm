package converter

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// MD2PDF converts markdown text to a PDF file.
// Prefer MDFile2PDF when the markdown references local assets such as images.
func MD2PDF(markdown, title, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir output dir: %w", err)
	}

	mdPath := filepath.Join(outputDir, "_article.md")
	if err := os.WriteFile(mdPath, []byte(markdown), 0o644); err != nil {
		return "", fmt.Errorf("write markdown: %w", err)
	}
	return MDFile2PDF(mdPath, title, outputDir)
}

// MDFile2PDF converts a Markdown file to a PDF file via pandoc.
// Relative image links are resolved from the Markdown file's directory.
func MDFile2PDF(markdownPath, title, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir output dir: %w", err)
	}
	if _, err := os.Stat(markdownPath); err != nil {
		return "", fmt.Errorf("markdown not found: %w", err)
	}

	slug := slugify(title)
	if slug == "" {
		slug = time.Now().UTC().Format("20060102-150405")
	}
	pdfPath := filepath.Join(outputDir, slug+".pdf")
	markdownDir := filepath.Dir(markdownPath)

	cmd := exec.Command(
		"pandoc",
		markdownPath,
		"--from", "gfm",
		"--standalone",
		"--pdf-engine", "xelatex",
		"--resource-path", markdownDir,
		"--metadata", "title="+title,
		"-V", "geometry:margin=15mm",
		"-V", "colorlinks=true",
		"-V", "urlcolor=blue",
	)
	cmd.Args = append(cmd.Args, pandocFontArgs()...)
	cmd.Args = append(cmd.Args, "-o", pdfPath)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return "", fmt.Errorf("pandoc failed (install pandoc and xelatex): %w", err)
		}
		return "", fmt.Errorf("pandoc failed (install pandoc and xelatex): %w: %s", err, msg)
	}

	if _, err := os.Stat(pdfPath); err != nil {
		return "", fmt.Errorf("pdf not generated: %w", err)
	}
	return pdfPath, nil
}

func pandocFontArgs() []string {
	args := make([]string, 0, 8)
	if font := firstAvailableFont("PingFang SC", "Noto Sans CJK SC", "Arial Unicode MS", "Songti SC"); font != "" {
		args = append(args, "-V", "CJKmainfont="+font)
	}
	if font := firstAvailableFont("Menlo", "DejaVu Sans Mono", "Arial Unicode MS"); font != "" {
		args = append(args, "-V", "monofont="+font)
	}
	return args
}

func firstAvailableFont(candidates ...string) string {
	if _, err := exec.LookPath("fc-match"); err != nil {
		return ""
	}
	for _, candidate := range candidates {
		out, err := exec.Command("fc-match", candidate).Output()
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(string(out)), strings.ToLower(candidate)) {
			return candidate
		}
	}
	return ""
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
