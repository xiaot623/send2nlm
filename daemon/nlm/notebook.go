package nlm

import (
	"context"
	"fmt"
	"time"

	"send2nlm/core"
)

// nbItem matches the JSON output of `notebooklm list --json`.
type nbItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	IsOwner   bool   `json:"is_owner"`
	CreatedAt string `json:"created_at"`
	URL       string `json:"url"`
	Emoji     string `json:"emoji"`
}

type nbListWrapper struct {
	Notebooks []nbItem `json:"notebooks"`
}

// ListNotebooks returns all notebooks from NotebookLM via `notebooklm list --json`.
func ListNotebooks(ctx context.Context) ([]core.Notebook, error) {
	out, err := execNotebookLM(ctx, "list", "--json")
	if err != nil {
		return nil, err
	}

	var items []nbItem
	if err := decodeJSON(out, &items); err == nil {
		return toNotebooks(items), nil
	}

	var wrapper nbListWrapper
	if err := decodeJSON(out, &wrapper); err == nil {
		return toNotebooks(wrapper.Notebooks), nil
	}

	return nil, fmt.Errorf("decode notebook list failed: unexpected output format")
}

func toNotebooks(items []nbItem) []core.Notebook {
	now := time.Now().UTC()
	notebooks := make([]core.Notebook, 0, len(items))
	for _, item := range items {
		emoji := item.Emoji
		if emoji == "" {
			emoji = "📒"
		}
		notebooks = append(notebooks, core.Notebook{
			ID:        item.ID,
			Title:     item.Title,
			IsOwner:   item.IsOwner,
			CreatedAt: item.CreatedAt,
			URL:       item.URL,
			Emoji:     emoji,
			CachedAt:  now,
		})
	}
	return notebooks
}

// nbCreateResponse matches the JSON output of `notebooklm create --use --json`.
// The 'notebook' field is nested; active_notebook_id is at the top level.
type nbCreateResponse struct {
	Notebook         nbCreateNotebook `json:"notebook"`
	ActiveNotebookID string           `json:"active_notebook_id"`
}

type nbCreateNotebook struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
}

// CreateNotebook creates a new notebook via `notebooklm create --use --json`.
func CreateNotebook(ctx context.Context, title, emoji string) (*core.Notebook, error) {
	if emoji == "" {
		emoji = "📒"
	}
	out, err := execNotebookLM(ctx, "create", title, "--use", "--json")
	if err != nil {
		return nil, err
	}
	var resp nbCreateResponse
	if err := decodeJSON(out, &resp); err != nil {
		return nil, fmt.Errorf("decode notebook create: %w", err)
	}

	id := resp.Notebook.ID
	if id == "" {
		id = resp.ActiveNotebookID
	}
	if id == "" {
		return nil, fmt.Errorf("create notebook returned no id")
	}

	now := time.Now().UTC()
	return &core.Notebook{
		ID:        id,
		Title:     resp.Notebook.Title,
		CreatedAt: resp.Notebook.CreatedAt,
		Emoji:     emoji,
		CachedAt:  now,
	}, nil
}
