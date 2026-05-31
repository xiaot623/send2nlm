// Lark Receiver — delivers pipeline artifacts through lark-cli as a bot.
// Placed in ~/.send2nlm/receiver/ and run as a compiled external plugin.
//
// Configuration (in ~/.send2nlm/config.json):
//   {
//     "receivers": {
//       "lark": {
//         "enabled": true
//       }
//     }
//   }
//
// The receiver sends to the current lark-cli user.
// Prerequisites: lark-cli must be installed and authenticated for user and bot usage.

package receiver

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"send2nlm/sdk"
)

// LarkReceiver delivers artifacts via lark-cli IM shortcuts.
type LarkReceiver struct{}

func (r *LarkReceiver) Name() string { return "lark" }

func (r *LarkReceiver) Receive(ctx context.Context, resources []sdk.Resource) error {
	cfg := sdk.LoadConfig()
	rc, ok := cfg.Receivers["lark"]
	if !ok {
		return nil // not configured, skip silently
	}

	enabled, _ := rc["enabled"].(bool)
	if !enabled {
		return nil
	}

	userID, err := larkCurrentUserID(ctx)
	if err != nil {
		return err
	}

	sourceURL := ""
	notebookTitle := ""
	notebookURL := ""
	if len(resources) > 0 {
		sourceURL = resources[0].SourceURL
		notebookTitle = resources[0].NotebookTitle
		notebookURL = resources[0].NotebookURL
	}

	msg := larkSummaryMarkdown(notebookTitle, notebookURL, sourceURL, resources)
	if err := larkSendMessage(ctx, userID, "--markdown", msg); err != nil {
		return fmt.Errorf("lark message: %w", err)
	}

	for _, res := range resources {
		if res.AssetPath == "" {
			continue
		}
		if err := larkSendFile(ctx, userID, res.AssetPath); err != nil {
			return fmt.Errorf("lark file %s: %w", res.TaskType, err)
		}
	}

	return nil
}

func larkSummaryMarkdown(notebookTitle, notebookURL, sourceURL string, resources []sdk.Resource) string {
	var b strings.Builder
	b.WriteString("**Send2NLM Job Complete**\n")
	if notebookTitle != "" {
		b.WriteString("- Notebook: ")
		b.WriteString(notebookTitle)
		b.WriteByte('\n')
	}
	if notebookURL != "" {
		b.WriteString("- Notebook URL: ")
		b.WriteString(notebookURL)
		b.WriteByte('\n')
	}
	if sourceURL != "" {
		b.WriteString("- Source: ")
		b.WriteString(sourceURL)
		b.WriteByte('\n')
	}
	if len(resources) > 0 {
		b.WriteString("- Artifacts:")
		for _, res := range resources {
			name := res.DeliveryName
			if name == "" {
				name = filepath.Base(res.AssetPath)
			}
			if name == "" {
				name = res.TaskType
			}
			b.WriteString("\n  - ")
			b.WriteString(name)
		}
	}
	return b.String()
}

func larkCurrentUserID(ctx context.Context) (string, error) {
	out, err := runLarkCLIOutput(ctx, "", "contact", "+get-user", "--as", "user", "--format", "json")
	if err != nil {
		return "", fmt.Errorf("lark current user: %w", err)
	}
	var payload any
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("lark current user decode: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	userID := findStringField(payload, "open_id")
	if userID == "" {
		return "", fmt.Errorf("lark current user: open_id not found")
	}
	return userID, nil
}

func findStringField(value any, key string) string {
	switch v := value.(type) {
	case map[string]any:
		if s, ok := v[key].(string); ok && s != "" {
			return s
		}
		for _, child := range v {
			if s := findStringField(child, key); s != "" {
				return s
			}
		}
	case []any:
		for _, child := range v {
			if s := findStringField(child, key); s != "" {
				return s
			}
		}
	}
	return ""
}

func larkSendMessage(ctx context.Context, userID, contentFlag, content string) error {
	args := []string{"im", "+messages-send", "--as", "bot", contentFlag, content}
	args = append(args, "--user-id", userID)
	return runLarkCLI(ctx, "", args...)
}

func larkSendFile(ctx context.Context, userID, path string) error {
	// lark-cli rejects absolute paths for --file. Run it from the file's
	// directory and pass a plain relative filename.
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	if strings.Contains(name, "..") {
		return fmt.Errorf("unsafe file name %q", name)
	}

	args := []string{"im", "+messages-send", "--as", "bot", "--file", name}
	args = append(args, "--user-id", userID)
	return runLarkCLI(ctx, dir, args...)
}

func runLarkCLI(ctx context.Context, dir string, args ...string) error {
	_, err := runLarkCLIOutput(ctx, dir, args...)
	return err
}

func runLarkCLIOutput(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "lark-cli", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("lark-cli %s failed: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// Receiver is the exported variable required by the external plugin wrapper.
var Receiver sdk.Receiver = &LarkReceiver{}
