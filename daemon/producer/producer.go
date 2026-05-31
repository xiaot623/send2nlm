package producer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"send2nlm/core"
)

var feishuPattern = regexp.MustCompile(`https?://[^/]*\.feishu\.cn/(wiki|docx|docs?)/`)

type inspectResponse struct {
	OK   bool `json:"ok"`
	Data struct {
		Title string `json:"title"`
		Token string `json:"token"`
		Type  string `json:"type"`
	} `json:"data"`
}

func ExportURLToPDF(ctx context.Context, cfg core.RuntimeConfig, url string) (string, error) {
	if feishuPattern.MatchString(url) {
		return exportFeishuToPDF(ctx, cfg, url)
	}
	return "", fmt.Errorf("unsupported URL for 0.0.1 producer: %s", url)
}

func exportFeishuToPDF(ctx context.Context, cfg core.RuntimeConfig, url string) (string, error) {
	out, err := nlmExec(ctx, "lark-cli", "drive", "+inspect", "--url", url)
	if err != nil {
		return "", err
	}
	var resp inspectResponse
	if err := json.Unmarshal(jsonPayload(out), &resp); err != nil {
		return "", fmt.Errorf("decode lark inspect: %w", err)
	}
	if !resp.OK || resp.Data.Token == "" || resp.Data.Type == "" {
		return "", fmt.Errorf("lark inspect did not return token/type")
	}

	jobDir := filepath.Join(cfg.TempDir(), time.Now().UTC().Format("20060102-150405"))
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return "", err
	}

	out, err = nlmExecInDir(ctx, jobDir, "lark-cli", "drive", "+export",
		"--token", resp.Data.Token,
		"--doc-type", resp.Data.Type,
		"--file-extension", "pdf",
		"--output-dir", ".",
		"--overwrite",
	)
	if err != nil && strings.Contains(string(out), "export artifact is already ready") {
		// keep the original error if parsing the hint later fails
	}
	if err == nil {
		matches, _ := filepath.Glob(filepath.Join(jobDir, "*.pdf"))
		if len(matches) > 0 {
			return matches[0], nil
		}
	}

	fileToken := exportDownloadToken(string(out))
	if fileToken == "" {
		return "", fmt.Errorf("export pdf failed: %s", strings.TrimSpace(string(out)))
	}
	out, err = nlmExecInDir(ctx, jobDir, "lark-cli", "drive", "+export-download",
		"--file-token", fileToken,
		"--file-name", "source.pdf",
		"--output-dir", ".",
		"--overwrite",
	)
	if err != nil {
		return "", err
	}

	var download struct {
		OK   bool `json:"ok"`
		Data struct {
			SavedPath string `json:"saved_path"`
		} `json:"data"`
	}
	if err := json.Unmarshal(jsonPayload(out), &download); err != nil {
		return "", fmt.Errorf("decode export-download: %w", err)
	}
	if !download.OK || download.Data.SavedPath == "" {
		return "", fmt.Errorf("export-download returned no saved_path")
	}
	return download.Data.SavedPath, nil
}

func exportDownloadToken(output string) string {
	re := regexp.MustCompile(`file_token[="\s:]+\"?([A-Za-z0-9]+)`)
	match := re.FindStringSubmatch(output)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}


