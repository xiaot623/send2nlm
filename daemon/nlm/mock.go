package nlm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type mockBackend struct {
	statePath   string
	artifactDir string
	mu          sync.Mutex
}

type mockState struct {
	Notebooks    []nbItem            `json:"notebooks"`
	Sources      map[string][]Source `json:"sources"`
	Tasks        map[string]mockTask `json:"tasks"`
	NextNotebook int                 `json:"next_notebook"`
	NextSource   int                 `json:"next_source"`
	NextTask     int                 `json:"next_task"`
}

type mockTask struct {
	TaskID     string   `json:"task_id"`
	NotebookID string   `json:"notebook_id"`
	TaskType   string   `json:"task_type"`
	SourceIDs  []string `json:"source_ids"`
	Status     string   `json:"status"`
	CreatedAt  string   `json:"created_at"`
}

func newMockBackend(statePath, artifactDir string) (*mockBackend, error) {
	if statePath == "" {
		return nil, fmt.Errorf("mock NotebookLM state path is required")
	}
	if artifactDir == "" {
		artifactDir = filepath.Join(filepath.Dir(statePath), "mock-artifacts")
	}
	if _, err := os.Stat(statePath); err != nil {
		return nil, fmt.Errorf("mock NotebookLM state not available: %w", err)
	}
	return &mockBackend{statePath: statePath, artifactDir: artifactDir}, nil
}

func (m *mockBackend) IsMock() bool { return true }

func (m *mockBackend) Exec(ctx context.Context, args ...string) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.load()
	if err != nil {
		return nil, err
	}

	if len(args) == 0 {
		return nil, fmt.Errorf("mock notebooklm: missing command")
	}

	switch args[0] {
	case "status":
		return encodeMockJSON(map[string]any{"status": "ok", "mock": true})
	case "list":
		return encodeMockJSON(state.Notebooks)
	case "create":
		return m.createNotebook(state, args[1:])
	case "source":
		return m.sourceCommand(state, args[1:])
	case "generate":
		return m.generateTask(state, args[1:])
	case "artifact":
		return m.artifactCommand(state, args[1:])
	case "download":
		return m.downloadArtifact(state, args[1:])
	default:
		return nil, fmt.Errorf("mock notebooklm: unsupported command %q", args[0])
	}
}

func (m *mockBackend) createNotebook(state mockState, args []string) ([]byte, error) {
	title := firstPositional(args)
	if title == "" {
		return nil, fmt.Errorf("mock notebooklm create: title is required")
	}
	state.NextNotebook++
	id := fmt.Sprintf("mock-nb-%03d", state.NextNotebook)
	now := time.Now().UTC().Format(time.RFC3339)
	item := nbItem{
		ID:        id,
		Title:     title,
		IsOwner:   true,
		CreatedAt: now,
		URL:       notebookURL(id),
		Emoji:     "📒",
	}
	state.Notebooks = append(state.Notebooks, item)
	if state.Sources == nil {
		state.Sources = map[string][]Source{}
	}
	state.Sources[id] = []Source{}
	if err := m.save(state); err != nil {
		return nil, err
	}
	return encodeMockJSON(nbCreateResponse{
		Notebook: nbCreateNotebook{
			ID:        item.ID,
			Title:     item.Title,
			CreatedAt: item.CreatedAt,
		},
		ActiveNotebookID: item.ID,
	})
}

func (m *mockBackend) sourceCommand(state mockState, args []string) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("mock notebooklm source: missing subcommand")
	}
	switch args[0] {
	case "add":
		notebookID := flagValue(args[1:], "-n", "--notebook")
		filePath := sourceAddPath(args[1:])
		if notebookID == "" || filePath == "" {
			return nil, fmt.Errorf("mock notebooklm source add: notebook and file are required")
		}
		if !state.hasNotebook(notebookID) {
			return nil, fmt.Errorf("mock notebooklm source add: notebook %q not found", notebookID)
		}
		state.NextSource++
		source := Source{
			ID:    fmt.Sprintf("mock-src-%03d", state.NextSource),
			Title: filepath.Base(filePath),
			Type:  "file",
		}
		if state.Sources == nil {
			state.Sources = map[string][]Source{}
		}
		state.Sources[notebookID] = append(state.Sources[notebookID], source)
		if err := m.save(state); err != nil {
			return nil, err
		}
		return encodeMockJSON(map[string]any{"source": source})
	case "list":
		notebookID := flagValue(args[1:], "-n", "--notebook")
		if notebookID == "" {
			return nil, fmt.Errorf("mock notebooklm source list: notebook is required")
		}
		return encodeMockJSON(state.Sources[notebookID])
	default:
		return nil, fmt.Errorf("mock notebooklm source: unsupported subcommand %q", args[0])
	}
}

func (m *mockBackend) generateTask(state mockState, args []string) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("mock notebooklm generate: task type is required")
	}
	cliType := args[0]
	taskType := taskTypeFromCLI(cliType)
	if taskType == "" {
		return nil, fmt.Errorf("mock notebooklm generate: unsupported task type %q", cliType)
	}
	notebookID := flagValue(args[1:], "-n", "--notebook")
	if notebookID == "" {
		return nil, fmt.Errorf("mock notebooklm generate: notebook is required")
	}
	if !state.hasNotebook(notebookID) {
		return nil, fmt.Errorf("mock notebooklm generate: notebook %q not found", notebookID)
	}
	state.NextTask++
	taskID := fmt.Sprintf("mock-task-%03d", state.NextTask)
	if state.Tasks == nil {
		state.Tasks = map[string]mockTask{}
	}
	state.Tasks[taskID] = mockTask{
		TaskID:     taskID,
		NotebookID: notebookID,
		TaskType:   taskType,
		SourceIDs:  allFlagValues(args[1:], "-s", "--source"),
		Status:     "pending",
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if err := m.save(state); err != nil {
		return nil, err
	}
	return encodeMockJSON(GenTaskResponse{
		TaskID: taskID,
		Status: "pending",
	})
}

func (m *mockBackend) artifactCommand(state mockState, args []string) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("mock notebooklm artifact: missing subcommand")
	}
	if args[0] != "poll" {
		return nil, fmt.Errorf("mock notebooklm artifact: unsupported subcommand %q", args[0])
	}
	taskID := lastPositional(args[1:])
	task, ok := state.Tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("mock notebooklm artifact poll: task %q not found", taskID)
	}
	task.Status = "completed"
	state.Tasks[taskID] = task
	if err := m.save(state); err != nil {
		return nil, err
	}
	return encodeMockJSON(PollResponse{
		TaskID: taskID,
		Status: "completed",
		Ready:  true,
	})
}

func (m *mockBackend) downloadArtifact(state mockState, args []string) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("mock notebooklm download: artifact type is required")
	}
	taskID := flagValue(args[1:], "-a", "--artifact")
	task, ok := state.Tasks[taskID]
	if taskID == "" || !ok {
		return nil, fmt.Errorf("mock notebooklm download: task %q not found", taskID)
	}
	outputPath := lastPositional(args[1:])
	if outputPath == "" {
		return nil, fmt.Errorf("mock notebooklm download: output path is required")
	}
	fixture, err := m.artifactFixture(task.TaskType)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(fixture)
	if err != nil {
		return nil, fmt.Errorf("mock notebooklm download: read fixture: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return nil, fmt.Errorf("mock notebooklm download: write output: %w", err)
	}
	return encodeMockJSON(map[string]any{"status": "ok", "path": outputPath})
}

func (m *mockBackend) load() (mockState, error) {
	data, err := os.ReadFile(m.statePath)
	if err != nil {
		return mockState{}, err
	}
	var state mockState
	if err := json.Unmarshal(data, &state); err != nil {
		return mockState{}, fmt.Errorf("mock NotebookLM state decode: %w", err)
	}
	if state.Sources == nil {
		state.Sources = map[string][]Source{}
	}
	if state.Tasks == nil {
		state.Tasks = map[string]mockTask{}
	}
	return state, nil
}

func (m *mockBackend) save(state mockState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(m.statePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(m.statePath, data, 0o644)
}

func (m *mockBackend) artifactFixture(taskType string) (string, error) {
	name := ""
	switch taskType {
	case "audio_overview":
		name = "audio_overview.wav"
	case "slide_deck":
		name = "slide_deck.pdf"
	case "video_overview":
		name = "video_overview.mp4"
	}
	if name == "" {
		return "", fmt.Errorf("unsupported artifact type %q", taskType)
	}
	return filepath.Join(m.artifactDir, name), nil
}

func (s mockState) hasNotebook(id string) bool {
	for _, notebook := range s.Notebooks {
		if notebook.ID == id {
			return true
		}
	}
	return false
}

func encodeMockJSON(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func flagValue(args []string, names ...string) string {
	for i := 0; i < len(args)-1; i++ {
		if hasString(names, args[i]) {
			return args[i+1]
		}
	}
	return ""
}

func allFlagValues(args []string, names ...string) []string {
	var values []string
	for i := 0; i < len(args)-1; i++ {
		if hasString(names, args[i]) {
			values = append(values, args[i+1])
			i++
		}
	}
	return values
}

func firstPositional(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--json" || arg == "--use" || arg == "--force" || arg == "--follow-symlinks" {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			i++
			continue
		}
		return arg
	}
	return ""
}

func lastPositional(args []string) string {
	last := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--json" || arg == "--use" || arg == "--force" || arg == "--follow-symlinks" {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			i++
			continue
		}
		last = arg
	}
	return last
}

func sourceAddPath(args []string) string {
	return lastPositional(args)
}

func taskTypeFromCLI(cliType string) string {
	switch cliType {
	case "audio":
		return "audio_overview"
	case "slide-deck":
		return "slide_deck"
	case "video":
		return "video_overview"
	default:
		return ""
	}
}

func notebookURL(id string) string {
	return "https://notebooklm.google.com/notebook/" + id
}

func hasString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
