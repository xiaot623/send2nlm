package core

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const DefaultPort = 18923

type RuntimeConfig struct {
	Dev       bool
	ConfigDir string
	Port      int
}

func NewRuntimeConfig(dev bool) RuntimeConfig {
	if !dev && os.Getenv("SEND2NLM_DEV") == "1" {
		dev = true
	}

	cwd, _ := os.Getwd()
	configDir := filepath.Join(userHomeDir(), ".send2nlm")
	if dev {
		configDir = filepath.Join(filepath.Dir(cwd), "dev_assets")
	}

	return RuntimeConfig{
		Dev:       dev,
		ConfigDir: configDir,
		Port:      DefaultPort,
	}
}

func (c RuntimeConfig) Ensure() error {
	for _, dir := range []string{
		c.ConfigDir,
		c.ProducerDir(),
		c.ReceiverDir(),
		c.TempDir(),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (c RuntimeConfig) ProducerDir() string { return filepath.Join(c.ConfigDir, "producer") }
func (c RuntimeConfig) ReceiverDir() string { return filepath.Join(c.ConfigDir, "receiver") }
func (c RuntimeConfig) TempDir() string     { return filepath.Join(c.ConfigDir, "tmp") }
func (c RuntimeConfig) DBPath() string      { return filepath.Join(c.ConfigDir, "send2nlm.db") }
func (c RuntimeConfig) PortFile() string    { return filepath.Join(c.ConfigDir, "daemon.port") }
func (c RuntimeConfig) PIDFile() string     { return filepath.Join(c.ConfigDir, "daemon.pid") }
func (c RuntimeConfig) ConfigFile() string  { return filepath.Join(c.ConfigDir, "config.json") }

type AppConfig struct {
	Receivers map[string]ReceiverConfig `json:"receivers"`
}

type ReceiverConfig struct {
	Enabled  bool   `json:"enabled"`
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

func LoadAppConfig(cfg RuntimeConfig) (AppConfig, error) {
	app := AppConfig{
		Receivers: map[string]ReceiverConfig{
			"download": {Enabled: true},
		},
	}
	data, err := os.ReadFile(cfg.ConfigFile())
	if err != nil {
		if os.IsNotExist(err) {
			return app, nil
		}
		return AppConfig{}, err
	}
	if err := json.Unmarshal(data, &app); err != nil {
		return AppConfig{}, err
	}
	if _, ok := app.Receivers["download"]; !ok {
		app.Receivers["download"] = ReceiverConfig{Enabled: true}
	}
	return app, nil
}

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}
