// Weixin Producer — converts WeChat Official Account article URLs to PDF via opencli.
// Placed in ~/.send2nlm/producer/ and run as a compiled external plugin.
//
// Prerequisites: opencli must be installed with the weixin site adapter available.

package producer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"send2nlm/converter"
	"send2nlm/sdk"
)

// WeixinProducer converts mp.weixin.qq.com article URLs to PDF.
type WeixinProducer struct{}

func (p *WeixinProducer) Name() string { return "weixin" }

var weixinPattern = regexp.MustCompile(`^https?://mp\.weixin\.qq\.com/s(?:/|\?)`)

func (p *WeixinProducer) Match(url string) bool {
	return weixinPattern.MatchString(url)
}

func (p *WeixinProducer) Produce(ctx context.Context, url string) (string, error) {
	outputDir := filepath.Join(os.TempDir(), "send2nlm", "weixin-"+time.Now().UTC().Format("20060102-150405"))
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("weixin mkdir: %w", err)
	}

	article, err := downloadWeixinArticle(ctx, url, outputDir)
	if err != nil {
		return "", fmt.Errorf("weixin download: %w", err)
	}
	if err := normalizeWeixinImages(article.Saved); err != nil {
		return "", fmt.Errorf("weixin normalize images: %w", err)
	}

	title := strings.TrimSpace(article.Title)
	if title == "" {
		title = "Weixin Article"
	}

	pdfPath, err := converter.MDFile2PDF(article.Saved, title, outputDir)
	if err != nil {
		return "", fmt.Errorf("weixin md to pdf: %w", err)
	}

	return pdfPath, nil
}

type weixinArticle struct {
	Title       string `json:"title"`
	Author      string `json:"author"`
	PublishTime string `json:"publish_time"`
	Status      string `json:"status"`
	Size        string `json:"size"`
	Saved       string `json:"saved"`
}

func downloadWeixinArticle(ctx context.Context, url, outputDir string) (weixinArticle, error) {
	cmd := exec.CommandContext(ctx,
		"opencli",
		"weixin",
		"download",
		"--url", url,
		"--output", outputDir,
		"--download-images", "true",
		"-f", "json",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return weixinArticle{}, err
		}
		return weixinArticle{}, fmt.Errorf("%w: %s", err, msg)
	}

	var articles []weixinArticle
	if err := json.Unmarshal(extractWeixinJSON(stdout.Bytes()), &articles); err != nil {
		return weixinArticle{}, fmt.Errorf("parse opencli json: %w", err)
	}
	if len(articles) == 0 {
		return weixinArticle{}, fmt.Errorf("no article returned by opencli")
	}

	article := articles[0]
	if article.Status != "" && article.Status != "success" {
		return weixinArticle{}, fmt.Errorf("opencli returned status %q", article.Status)
	}
	if article.Saved == "" {
		return weixinArticle{}, fmt.Errorf("opencli did not return saved markdown path")
	}
	if !filepath.IsAbs(article.Saved) {
		article.Saved = filepath.Join(outputDir, article.Saved)
	}
	if _, err := os.Stat(article.Saved); err != nil {
		return weixinArticle{}, fmt.Errorf("opencli markdown not found: %w", err)
	}

	return article, nil
}

func normalizeWeixinImages(markdownPath string) error {
	markdown, err := os.ReadFile(markdownPath)
	if err != nil {
		return fmt.Errorf("read markdown: %w", err)
	}

	markdownDir := filepath.Dir(markdownPath)
	updated := string(markdown)
	err = filepath.WalkDir(markdownDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		isWebP, err := hasWebPMagic(path)
		if err != nil || !isWebP {
			return err
		}

		pngPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".png"
		if err := convertImageToPNG(path, pngPath); err != nil {
			return err
		}

		oldRel, err := filepath.Rel(markdownDir, path)
		if err != nil {
			return err
		}
		newRel, err := filepath.Rel(markdownDir, pngPath)
		if err != nil {
			return err
		}
		updated = strings.ReplaceAll(updated, filepath.ToSlash(oldRel), filepath.ToSlash(newRel))
		return nil
	})
	if err != nil {
		return err
	}

	if updated != string(markdown) {
		if err := os.WriteFile(markdownPath, []byte(updated), 0o644); err != nil {
			return fmt.Errorf("write markdown: %w", err)
		}
	}
	return nil
}

func hasWebPMagic(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	header := make([]byte, 12)
	n, err := file.Read(header)
	if err != nil && n == 0 {
		return false, err
	}
	return n >= 12 && string(header[0:4]) == "RIFF" && string(header[8:12]) == "WEBP", nil
}

func convertImageToPNG(src, dst string) error {
	if magick, err := exec.LookPath("magick"); err == nil {
		return runImageConvert(magick, src, dst)
	}
	if convert, err := exec.LookPath("convert"); err == nil {
		return runImageConvert(convert, src, dst)
	}
	if sips, err := exec.LookPath("sips"); err == nil {
		cmd := exec.Command(sips, "-s", "format", "png", src, "--out", dst)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("sips convert %s: %w: %s", filepath.Base(src), err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	return fmt.Errorf("cannot convert WebP image %s: install ImageMagick or sips", filepath.Base(src))
}

func runImageConvert(binary, src, dst string) error {
	cmd := exec.Command(binary, src, dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s convert %s: %w: %s", filepath.Base(binary), filepath.Base(src), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// extractWeixinJSON tolerates update notices or other CLI chatter around JSON.
func extractWeixinJSON(raw []byte) []byte {
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

// Producer is the exported variable required by the external plugin wrapper.
var Producer sdk.Producer = &WeixinProducer{}
