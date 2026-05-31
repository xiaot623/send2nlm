// Lark Producer — converts Feishu/Lark docx URLs to PDF via lark-cli.
// Placed in ~/.send2nlm/producer/ and loaded by the yaegi script engine.
//
// Prerequisites: lark-cli must be installed and authenticated.
// See: https://github.com/earendil-works/lark-cli

package producer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"send2nlm/sdk"
)

// LarkProducer converts Feishu/Lark document URLs to PDF.
type LarkProducer struct{}

func (p *LarkProducer) Name() string { return "lark" }

var feishuPattern = regexp.MustCompile(`https?://[^/]*\.(feishu|feishu\.cn|larksuite|larkoffice)\.(com|cn)/(docx|docs?|wiki)/([A-Za-z0-9]+)`)

func (p *LarkProducer) Match(url string) bool {
	return feishuPattern.MatchString(url)
}

func (p *LarkProducer) Produce(ctx context.Context, url string) (string, error) {
	// 1. Inspect the document to get token and type
	inspectOut, err := runLarkCLI(ctx, "", "drive", "+inspect", "--url", url)
	if err != nil {
		return "", fmt.Errorf("lark inspect: %w", err)
	}

	var inspectResp struct {
		OK   bool `json:"ok"`
		Data struct {
			Title string `json:"title"`
			Token string `json:"token"`
			Type  string `json:"type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(extractJSON(inspectOut), &inspectResp); err != nil {
		return "", fmt.Errorf("lark inspect decode: %w", err)
	}
	if !inspectResp.OK || inspectResp.Data.Token == "" || inspectResp.Data.Type == "" {
		return "", fmt.Errorf("lark inspect did not return token/type: %s", inspectOut)
	}

	// 2. Create a job directory for the export
	jobDir := filepath.Join("/tmp/send2nlm", time.Now().UTC().Format("20060102-150405"))
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return "", fmt.Errorf("lark mkdir: %w", err)
	}

	// 3. Export to PDF
	exportOut, err := runLarkCLI(ctx, jobDir, "drive", "+export",
		"--token", inspectResp.Data.Token,
		"--doc-type", inspectResp.Data.Type,
		"--file-extension", "pdf",
		"--output-dir", ".",
		"--overwrite",
	)

	// 3a. If export succeeded and we have a PDF file, return it
	if err == nil {
		matches, _ := filepath.Glob(filepath.Join(jobDir, "*.pdf"))
		if len(matches) > 0 {
			return matches[0], nil
		}
	}

	// 3b. If the artifact was already cached, extract the file_token and download
	if strings.Contains(string(exportOut), "export artifact is already ready") || err != nil {
		fileToken := extractFileToken(string(exportOut))
		if fileToken == "" {
			return "", fmt.Errorf("lark export failed: %s", strings.TrimSpace(string(exportOut)))
		}

		dlOut, err := runLarkCLI(ctx, jobDir, "drive", "+export-download",
			"--file-token", fileToken,
			"--file-name", "source.pdf",
			"--output-dir", ".",
			"--overwrite",
		)
		if err != nil {
			return "", fmt.Errorf("lark export-download: %w\n%s", err, dlOut)
		}

		var dlResp struct {
			OK   bool `json:"ok"`
			Data struct {
				SavedPath string `json:"saved_path"`
			} `json:"data"`
		}
		if err := json.Unmarshal(extractJSON(dlOut), &dlResp); err != nil {
			return "", fmt.Errorf("lark export-download decode: %w", err)
		}
		if !dlResp.OK || dlResp.Data.SavedPath == "" {
			return "", fmt.Errorf("lark export-download: no saved_path in response")
		}
		return dlResp.Data.SavedPath, nil
	}

	return "", fmt.Errorf("lark export: unexpected state")
}

// runLarkCLI executes lark-cli with the given arguments.
func runLarkCLI(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "lark-cli", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.CombinedOutput()
}

// extractJSON finds the first JSON object or array in raw CLI output.
func extractJSON(raw []byte) []byte {
	s := string(raw)
	start := strings.IndexAny(s, "[{")
	if start < 0 {
		return raw
	}
	return []byte(s[start:])
}

// extractFileToken pulls the file_token from lark-cli output.
func extractFileToken(output string) string {
	re := regexp.MustCompile(`file_token[="\s:]+\"?([A-Za-z0-9]+)`)
	match := re.FindStringSubmatch(output)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

// Producer is the exported variable required by the script engine.
var Producer sdk.Producer = &LarkProducer{}
