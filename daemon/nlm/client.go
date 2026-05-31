package nlm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// execNotebookLM runs the notebooklm-py CLI with the given arguments.
// All commands are expected to return JSON output (--json flag is handled per-command).
func execNotebookLM(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "notebooklm", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("notebooklm %v failed: %w\nstderr: %s", args, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// CheckNotebookLMAvailable verifies the notebooklm CLI is installed and authenticated.
func CheckNotebookLMAvailable(ctx context.Context) error {
	_, err := execNotebookLM(ctx, "status", "--json")
	return err
}

// decodeJSON extracts and decodes the first JSON object or array from raw CLI output.
func decodeJSON[T any](raw []byte, out *T) error {
	start := strings.IndexAny(string(raw), "[{")
	if start < 0 {
		return fmt.Errorf("no JSON payload found in output: %s", string(raw))
	}
	dec := json.NewDecoder(strings.NewReader(string(raw[start:])))
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("decode failed: %w\nraw: %s", err, string(raw))
	}
	return nil
}
