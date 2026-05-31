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

// Source represents a single resource in a notebook.
type Source struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// srcListWrapper matches the JSON output format of list if it wraps in "sources".
type srcListWrapper struct {
	Sources []Source `json:"sources"`
}

// ListSources retrieves all sources for a notebook via `notebooklm source list --json`.
func ListSources(ctx context.Context, notebookID string) ([]Source, error) {
	out, err := execNotebookLM(ctx, "source", "list", "-n", notebookID, "--json")
	if err != nil {
		return nil, err
	}

	var sources []Source
	if err := decodeJSON(out, &sources); err == nil {
		return sources, nil
	}

	var wrapper srcListWrapper
	if err := decodeJSON(out, &wrapper); err == nil {
		return wrapper.Sources, nil
	}

	return nil, fmt.Errorf("decode source list failed: unexpected output format")
}
