package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"send2nlm/core"
)

func (s *Store) CreateJob(job *core.Job) error {
	_, err := s.db.Exec(`
INSERT INTO jobs(id, notebook_id, notebook_title, url, status, tasks, source_ids, task_results, error, retry_count, created_at, updated_at, completed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.NotebookID, job.NotebookTitle, job.URL, job.Status, core.MarshalTasks(job.Tasks), core.MarshalTasks(job.SourceIDs), core.MarshalTaskResults(job.TaskResults), job.Error, job.RetryCount, job.CreatedAt, job.UpdatedAt, job.CompletedAt)
	return err
}

func (s *Store) GetJob(ctx context.Context, id string) (*core.Job, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, notebook_id, notebook_title, url, status, tasks, source_ids, task_results, error, retry_count, created_at, updated_at, completed_at
FROM jobs WHERE id = ?`, id)
	return scanJob(row)
}

func (s *Store) ListJobs(ctx context.Context, statusFilter string) ([]core.Job, error) {
	query := `
SELECT id, notebook_id, notebook_title, url, status, tasks, source_ids, task_results, error, retry_count, created_at, updated_at, completed_at
FROM jobs`
	var args []any
	if statusFilter != "" {
		query += ` WHERE status = ?`
		args = append(args, statusFilter)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []core.Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}
	return jobs, rows.Err()
}

func (s *Store) UpdateJobStatus(ctx context.Context, id, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE jobs SET status = ?, updated_at = ? WHERE id = ?`, status, time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (s *Store) UpdateJobProgress(ctx context.Context, id string, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	parts := make([]string, 0, len(updates)+1)
	args := make([]any, 0, len(updates)+2)
	for key, value := range updates {
		parts = append(parts, fmt.Sprintf("%s = ?", key))
		args = append(args, value)
	}
	parts = append(parts, "updated_at = ?")
	args = append(args, time.Now().UTC().Format(time.RFC3339), id)
	query := `UPDATE jobs SET ` + strings.Join(parts, ", ") + ` WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

func (s *Store) GetPendingJobs(ctx context.Context) ([]core.Job, error) {
	return s.ListJobs(ctx, "")
}

type scanner interface {
	Scan(dest ...any) error
}

func scanJob(row scanner) (*core.Job, error) {
	var (
		job             core.Job
		tasksJSON       string
		sourceIDsJSON   string
		taskResultsJSON string
	)
	err := row.Scan(
		&job.ID,
		&job.NotebookID,
		&job.NotebookTitle,
		&job.URL,
		&job.Status,
		&tasksJSON,
		&sourceIDsJSON,
		&taskResultsJSON,
		&job.Error,
		&job.RetryCount,
		&job.CreatedAt,
		&job.UpdatedAt,
		&job.CompletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(tasksJSON), &job.Tasks)
	_ = json.Unmarshal([]byte(sourceIDsJSON), &job.SourceIDs)
	if job.TaskResults == nil {
		job.TaskResults = map[string]core.TaskResult{}
	}
	_ = json.Unmarshal([]byte(taskResultsJSON), &job.TaskResults)
	return &job, nil
}
