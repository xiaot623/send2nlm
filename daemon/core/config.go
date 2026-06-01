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
		c.URLAspectDir(),
		c.ReceiveAspectDir(),
		c.TempDir(),
		c.PluginCacheDir(),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return c.ensureConfigFile()
}

func (c RuntimeConfig) ProducerDir() string { return filepath.Join(c.ConfigDir, "producer") }
func (c RuntimeConfig) ReceiverDir() string { return filepath.Join(c.ConfigDir, "receiver") }
func (c RuntimeConfig) AspectDir() string   { return filepath.Join(c.ConfigDir, "aspect") }
func (c RuntimeConfig) URLAspectDir() string {
	return filepath.Join(c.AspectDir(), "url")
}
func (c RuntimeConfig) ReceiveAspectDir() string {
	return filepath.Join(c.AspectDir(), "receive")
}
func (c RuntimeConfig) TempDir() string { return filepath.Join(c.ConfigDir, "tmp") }
func (c RuntimeConfig) PluginCacheDir() string {
	return filepath.Join(c.ConfigDir, "cache", "plugins")
}
func (c RuntimeConfig) DBPath() string     { return filepath.Join(c.ConfigDir, "send2nlm.db") }
func (c RuntimeConfig) PortFile() string   { return filepath.Join(c.ConfigDir, "daemon.port") }
func (c RuntimeConfig) PIDFile() string    { return filepath.Join(c.ConfigDir, "daemon.pid") }
func (c RuntimeConfig) ConfigFile() string { return filepath.Join(c.ConfigDir, "config.json") }

type AppConfig struct {
	Producers map[string]ProducerConfig  `json:"producers"`
	Receivers map[string]json.RawMessage `json:"receivers"`
}

type ProducerConfig struct {
	Enabled bool   `json:"enabled"`
	CLI     string `json:"cli,omitempty"`
}

type ReceiverConfig struct {
	Enabled  bool   `json:"enabled"`
	BotToken string `json:"bot_token,omitempty"`
	ChatID   string `json:"chat_id,omitempty"`
}

func LoadAppConfig(cfg RuntimeConfig) (AppConfig, error) {
	app := defaultAppConfig()
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
	if app.Producers == nil {
		app.Producers = map[string]ProducerConfig{}
	}
	if _, ok := app.Producers["lark"]; !ok {
		app.Producers["lark"] = ProducerConfig{Enabled: true, CLI: "lark-cli"}
	}
	if _, ok := app.Producers["weixin"]; !ok {
		app.Producers["weixin"] = ProducerConfig{Enabled: true, CLI: "opencli weixin"}
	}
	if app.Receivers == nil {
		app.Receivers = map[string]json.RawMessage{}
	}
	if _, ok := app.Receivers["lark"]; !ok {
		app.Receivers["lark"] = mustMarshalJSON(map[string]any{
			"enabled": false,
		})
	}
	return app, nil
}

func (c RuntimeConfig) ensureConfigFile() error {
	if _, err := os.Stat(c.ConfigFile()); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	data, err := json.MarshalIndent(defaultAppConfig(), "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(c.ConfigFile(), data, 0o600)
}

func mustMarshalJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

func defaultAppConfig() AppConfig {
	return AppConfig{
		Producers: map[string]ProducerConfig{
			"lark":   {Enabled: true, CLI: "lark-cli"},
			"weixin": {Enabled: true, CLI: "opencli weixin"},
		},
		Receivers: map[string]json.RawMessage{
			"download": mustMarshalJSON(ReceiverConfig{Enabled: true}),
			"lark": mustMarshalJSON(map[string]any{
				"enabled": false,
			}),
			"telegram": mustMarshalJSON(map[string]any{
				"enabled":   false,
				"bot_token": "",
				"chat_id":   "",
			}),
		},
	}
}

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}
