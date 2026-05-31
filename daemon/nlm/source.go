package nlm

import (
	"context"
	"fmt"
)

// srcAddResponse matches the JSON output of `notebooklm source add --json`.
// The source info is nested under the "source" key.
type srcAddResponse struct {
	Source struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Type  string `json:"type"`
	} `json:"source"`
}

// AddFileSource uploads a file to a notebook via `notebooklm source add -n <id> <file> --json`.
func AddFileSource(ctx context.Context, notebookID, filePath string) (string, error) {
	out, err := execNotebookLM(ctx, "source", "add", "-n", notebookID, "--follow-symlinks", filePath, "--json")
	if err != nil {
		return "", err
	}
	var resp srcAddResponse
	if err := decodeJSON(out, &resp); err != nil {
		return "", fmt.Errorf("decode source add: %w", err)
	}
	if resp.Source.ID == "" {
		return "", fmt.Errorf("source add returned no source_id")
	}
	return resp.Source.ID, nil
}
