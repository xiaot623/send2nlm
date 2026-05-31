package producer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

func nlmExec(ctx context.Context, bin string, args ...string) ([]byte, error) {
	return nlmExecInDir(ctx, "", bin, args...)
}

func nlmExecInDir(ctx context.Context, dir, bin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return append(stdout.Bytes(), stderr.Bytes()...), fmt.Errorf("%s %v failed: %w", bin, args, err)
	}
	if stdout.Len() == 0 {
		return stderr.Bytes(), nil
	}
	return stdout.Bytes(), nil
}

func jsonPayload(raw []byte) []byte {
	start := strings.IndexAny(string(raw), "[{")
	if start < 0 {
		return raw
	}
	return raw[start:]
}

// Helper used here to keep producer independent from nlm internals.
var _ = json.Unmarshal
