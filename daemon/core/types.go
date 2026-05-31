package core

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

const (
	StatusPending     = "pending"
	StatusTasking     = "tasking"
	StatusPolling     = "polling"
	StatusDownloading = "downloading"
	StatusReceiving   = "receiving"
	StatusDone        = "done"
	StatusFailed      = "failed"
)

type Notebook struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	IsOwner   bool      `json:"is_owner"`
	CreatedAt string    `json:"created_at"`
	URL       string    `json:"url"`
	Emoji     string    `json:"emoji,omitempty"`
	CachedAt  time.Time `json:"cached_at,omitempty"`
}

type TaskResult struct {
	TaskID      string `json:"task_id"`
	Status      string `json:"status"`
	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at"`
	AssetPath   string `json:"asset_path"`
}

type Job struct {
	ID            string                `json:"job_id"`
	NotebookID    string                `json:"notebook_id"`
	NotebookTitle string                `json:"notebook_title"`
	URL           string                `json:"url"`
	Status        string                `json:"status"`
	Tasks         []string              `json:"tasks"`
	SourceIDs     []string              `json:"source_ids"`
	TaskResults   map[string]TaskResult `json:"task_results"`
	Error         string                `json:"error"`
	RetryCount    int                   `json:"retry_count"`
	CreatedAt     string                `json:"created_at"`
	UpdatedAt     string                `json:"updated_at"`
	CompletedAt   string                `json:"completed_at"`
}

func NewJob(notebookID, notebookTitle, url string, tasks []string, sourceIDs []string) *Job {
	now := time.Now().UTC().Format(time.RFC3339)
	return &Job{
		ID:            newID(),
		NotebookID:    notebookID,
		NotebookTitle: notebookTitle,
		URL:           url,
		Tasks:         tasks,
		SourceIDs:     sourceIDs,
		Status:        StatusPending,
		TaskResults:   map[string]TaskResult{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func ParseTaskList(tasksCSV string) []string {
	if tasksCSV == "" {
		return nil
	}
	parts := strings.Split(tasksCSV, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func MarshalTasks(tasks []string) string {
	data, _ := json.Marshal(tasks)
	return string(data)
}

func MarshalTaskResults(results map[string]TaskResult) string {
	if results == nil {
		results = map[string]TaskResult{}
	}
	data, _ := json.Marshal(results)
	return string(data)
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405")
	}
	return hex.EncodeToString(b[:])
}
