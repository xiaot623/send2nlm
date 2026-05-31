package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

// Resource is a single artifact produced by the pipeline.
type Resource struct {
	TaskType      string // "audio_overview" | "slide_deck"
	AssetPath     string // local file path
	DeliveryName  string // pre-computed filename (e.g. "Research_Notes_audio.wav")
	MimeType      string // e.g. "audio/wav", "application/pdf"
	NotebookTitle string
	NotebookURL   string
	SourceURL     string
}

// Config is the open configuration structure available to scripts.
// Scripts can only read configuration; writing is handled by daemon/CLI.
type Config struct {
	Receivers map[string]map[string]interface{} `json:"receivers"`
}

// Receiver is the artifact delivery adapter interface.
// Each script file must export a package-level variable named "Receiver"
// whose type implements this interface.
type Receiver interface {
	Name() string
	Receive(ctx context.Context, resources []Resource) error
}

// TaskSuffix maps a pipeline task type to a short filename suffix.
func TaskSuffix(taskType string) string {
	switch taskType {
	case "audio_overview":
		return "audio"
	case "slide_deck":
		return "slide"
	default:
		return taskType
	}
}

// BuildDeliveryName computes a safe filename for a resource.
// Called once by the pipeline; receivers just read res.DeliveryName.
// Format: <NotebookTitle>_<suffix>.<ext>
func BuildDeliveryName(res Resource) string {
	title := sanitizeFilename(res.NotebookTitle)
	if title == "" {
		title = "notebook"
	}
	suffix := TaskSuffix(res.TaskType)
	ext := filepath.Ext(res.AssetPath)
	return fmt.Sprintf("%s_%s%s", title, suffix, ext)
}

// sanitizeFilename removes characters unsafe for cross-platform filenames.
func sanitizeFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if len(name) > 100 {
		name = name[:100]
	}
	return strings.TrimRight(name, ". ")
}

var (
	configDir   string
	configMu    sync.RWMutex
)

// SetConfigDir sets the configuration directory for LoadConfig.
// Called once during daemon startup.
func SetConfigDir(dir string) {
	configMu.Lock()
	defer configMu.Unlock()
	configDir = dir
}

// LoadConfig reads the application config from config.json.
// It is the function exported to scripts so they can read their own
// configuration section (e.g. sdk.LoadConfig().Receivers["telegram"]).
func LoadConfig() Config {
	configMu.RLock()
	dir := configDir
	configMu.RUnlock()

	if dir == "" {
		return Config{Receivers: map[string]map[string]interface{}{}}
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		return Config{Receivers: map[string]map[string]interface{}{}}
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{Receivers: map[string]map[string]interface{}{}}
	}
	return cfg
}
