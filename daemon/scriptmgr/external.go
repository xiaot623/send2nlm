package scriptmgr

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"send2nlm/sdk"

	"github.com/fsnotify/fsnotify"
)

type PluginKind string

const (
	ProducerPlugin      PluginKind = "producer"
	ReceiverPlugin      PluginKind = "receiver"
	URLAspectPlugin     PluginKind = "url_aspect"
	ReceiveAspectPlugin PluginKind = "receive_aspect"
)

type Loader struct {
	configDir string
	cacheDir  string
	moduleDir string
}

func NewLoader(configDir, cacheDir string) *Loader {
	cwd, _ := os.Getwd()
	moduleDir := findModuleDir(cwd)
	return &Loader{
		configDir: configDir,
		cacheDir:  cacheDir,
		moduleDir: moduleDir,
	}
}

func findModuleDir(start string) string {
	if _, err := os.Stat(filepath.Join(start, "daemon", "go.mod")); err == nil {
		return filepath.Join(start, "daemon")
	}
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start
		}
		dir = parent
	}
}

func LoadProducerDir(loader *Loader, registry *ProducerRegistry, dir string) {
	producers := evaluateProducerDir(loader, dir)
	registry.SetScripts(producers)
	log.Printf("[scriptmgr] loaded %d producer plugin(s) from %s", len(producers), dir)
}

func LoadReceiverDir(loader *Loader, registry *ReceiverRegistry, dir string) {
	receivers := evaluateReceiverDir(loader, dir)
	registry.SetScripts(receivers)
	log.Printf("[scriptmgr] loaded %d receiver plugin(s) from %s", len(receivers), dir)
}

func LoadURLAspectDir(loader *Loader, registry *URLAspectRegistry, dir string) {
	aspects := evaluateURLAspectDir(loader, dir)
	registry.SetScripts(aspects)
	log.Printf("[scriptmgr] loaded %d url aspect plugin(s) from %s", len(aspects), dir)
}

func LoadReceiveAspectDir(loader *Loader, registry *ReceiveAspectRegistry, dir string) {
	aspects := evaluateReceiveAspectDir(loader, dir)
	registry.SetScripts(aspects)
	log.Printf("[scriptmgr] loaded %d receive aspect plugin(s) from %s", len(aspects), dir)
}

func WatchProducerDir(loader *Loader, registry *ProducerRegistry, dir string) error {
	return watchDir(dir, func() {
		LoadProducerDir(loader, registry, dir)
	})
}

func WatchReceiverDir(loader *Loader, registry *ReceiverRegistry, dir string) error {
	return watchDir(dir, func() {
		LoadReceiverDir(loader, registry, dir)
	})
}

func WatchURLAspectDir(loader *Loader, registry *URLAspectRegistry, dir string) error {
	return watchDir(dir, func() {
		LoadURLAspectDir(loader, registry, dir)
	})
}

func WatchReceiveAspectDir(loader *Loader, registry *ReceiveAspectRegistry, dir string) error {
	return watchDir(dir, func() {
		LoadReceiveAspectDir(loader, registry, dir)
	})
}

func evaluateProducerDir(loader *Loader, dir string) []sdk.Producer {
	paths, err := pluginFiles(dir)
	if err != nil {
		log.Printf("[scriptmgr] cannot read producer dir %s: %v", dir, err)
		return nil
	}
	out := make([]sdk.Producer, 0, len(paths))
	for _, path := range paths {
		plugin, err := loader.compile(path, ProducerPlugin)
		if err != nil {
			log.Printf("[scriptmgr] skip producer %s: %v", filepath.Base(path), err)
			continue
		}
		p := &ExternalProducer{plugin: plugin}
		log.Printf("[scriptmgr] loaded producer %q from %s", p.Name(), filepath.Base(path))
		out = append(out, p)
	}
	return out
}

func evaluateReceiverDir(loader *Loader, dir string) []sdk.Receiver {
	paths, err := pluginFiles(dir)
	if err != nil {
		log.Printf("[scriptmgr] cannot read receiver dir %s: %v", dir, err)
		return nil
	}
	out := make([]sdk.Receiver, 0, len(paths))
	for _, path := range paths {
		plugin, err := loader.compile(path, ReceiverPlugin)
		if err != nil {
			log.Printf("[scriptmgr] skip receiver %s: %v", filepath.Base(path), err)
			continue
		}
		r := &ExternalReceiver{plugin: plugin}
		log.Printf("[scriptmgr] loaded receiver %q from %s", r.Name(), filepath.Base(path))
		out = append(out, r)
	}
	return out
}

func evaluateURLAspectDir(loader *Loader, dir string) []sdk.URLAspect {
	paths, err := pluginFiles(dir)
	if err != nil {
		log.Printf("[scriptmgr] cannot read url aspect dir %s: %v", dir, err)
		return nil
	}
	out := make([]sdk.URLAspect, 0, len(paths))
	for _, path := range paths {
		plugin, err := loader.compile(path, URLAspectPlugin)
		if err != nil {
			log.Printf("[scriptmgr] skip url aspect %s: %v", filepath.Base(path), err)
			continue
		}
		a := &ExternalURLAspect{plugin: plugin}
		log.Printf("[scriptmgr] loaded url aspect %q from %s", a.Name(), filepath.Base(path))
		out = append(out, a)
	}
	return out
}

func evaluateReceiveAspectDir(loader *Loader, dir string) []sdk.ReceiveAspect {
	paths, err := pluginFiles(dir)
	if err != nil {
		log.Printf("[scriptmgr] cannot read receive aspect dir %s: %v", dir, err)
		return nil
	}
	out := make([]sdk.ReceiveAspect, 0, len(paths))
	for _, path := range paths {
		plugin, err := loader.compile(path, ReceiveAspectPlugin)
		if err != nil {
			log.Printf("[scriptmgr] skip receive aspect %s: %v", filepath.Base(path), err)
			continue
		}
		a := &ExternalReceiveAspect{plugin: plugin}
		log.Printf("[scriptmgr] loaded receive aspect %q from %s", a.Name(), filepath.Base(path))
		out = append(out, a)
	}
	return out
}

func pluginFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	return paths, nil
}

func watchDir(dir string, reload func()) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := watcher.Add(dir); err != nil {
		_ = watcher.Close()
		return err
	}

	go func() {
		defer watcher.Close()
		var pending bool
		timer := time.NewTimer(time.Hour)
		if !timer.Stop() {
			<-timer.C
		}
		for {
			select {
			case evt, ok := <-watcher.Events:
				if !ok {
					return
				}
				if strings.HasSuffix(evt.Name, ".go") {
					pending = true
					timer.Reset(300 * time.Millisecond)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("[scriptmgr] watcher error: %v", err)
			case <-timer.C:
				if pending {
					reload()
					pending = false
				}
			}
		}
	}()

	return nil
}

type compiledPlugin struct {
	kind      PluginKind
	name      string
	priority  int
	binary    string
	configDir string
}

func (l *Loader) compile(path string, kind PluginKind) (*compiledPlugin, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(bytes.Join([][]byte{[]byte(kind), source}, []byte{0}))
	hash := hex.EncodeToString(sum[:])
	dir := filepath.Join(l.cacheDir, string(kind), hash)
	binary := filepath.Join(dir, "plugin")
	if _, err := os.Stat(binary); err != nil {
		if err := l.build(dir, binary, path, source, kind); err != nil {
			return nil, err
		}
	}
	plugin := &compiledPlugin{kind: kind, binary: binary, configDir: l.configDir}
	name, priority, err := plugin.metadata(context.Background())
	if err != nil {
		return nil, err
	}
	plugin.name = name
	plugin.priority = priority
	return plugin, nil
}

func (l *Loader) build(dir, binary, path string, source []byte, kind PluginKind) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	wrapped, err := wrapSource(source, kind)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.go"), wrapped, 0o644); err != nil {
		return err
	}
	mod := fmt.Sprintf("module send2nlm-plugin\n\nrequire send2nlm v0.0.0\n\nreplace send2nlm => %s\n", l.moduleDir)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-mod=mod", "-o", binary, ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build %s: %w\n%s", filepath.Base(path), err, strings.TrimSpace(string(out)))
	}
	return nil
}

var packageRe = regexp.MustCompile(`(?m)^package\s+\w+`)

func wrapSource(source []byte, kind PluginKind) ([]byte, error) {
	text := packageRe.ReplaceAllString(string(source), "package main")
	if text == string(source) {
		return nil, fmt.Errorf("plugin source has no package declaration")
	}
	switch kind {
	case ProducerPlugin:
		text += "\nfunc main() { sdk.ServeProducer(Producer) }\n"
	case ReceiverPlugin:
		text += "\nfunc main() { sdk.ServeReceiver(Receiver) }\n"
	case URLAspectPlugin:
		text += "\nfunc main() { sdk.ServeURLAspect(URLAspect) }\n"
	case ReceiveAspectPlugin:
		text += "\nfunc main() { sdk.ServeReceiveAspect(ReceiveAspect) }\n"
	default:
		return nil, fmt.Errorf("unknown plugin kind %q", kind)
	}
	return []byte(text), nil
}

type pluginRequest struct {
	Method    string         `json:"method"`
	URL       string         `json:"url,omitempty"`
	Resources []sdk.Resource `json:"resources,omitempty"`
}

type pluginResponse struct {
	Name      string         `json:"name,omitempty"`
	Priority  int            `json:"priority,omitempty"`
	Match     bool           `json:"match,omitempty"`
	PDFPath   string         `json:"pdf_path,omitempty"`
	URL       string         `json:"url,omitempty"`
	Resources []sdk.Resource `json:"resources,omitempty"`
	Error     string         `json:"error,omitempty"`
}

func (p *compiledPlugin) metadata(ctx context.Context) (string, int, error) {
	resp, err := p.call(ctx, pluginRequest{Method: "metadata"})
	if err != nil {
		return "", 0, err
	}
	if resp.Name == "" {
		return "", 0, fmt.Errorf("plugin returned empty name")
	}
	return resp.Name, resp.Priority, nil
}

func (p *compiledPlugin) call(ctx context.Context, req pluginRequest) (pluginResponse, error) {
	data, _ := json.Marshal(req)
	cmd := exec.CommandContext(ctx, p.binary)
	cmd.Stdin = bytes.NewReader(data)
	cmd.Env = append(os.Environ(), "SEND2NLM_CONFIG_DIR="+p.configDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var resp pluginResponse
	if decErr := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &resp); decErr != nil {
		if err != nil {
			return resp, fmt.Errorf("plugin %s failed: %w\n%s", p.binary, err, strings.TrimSpace(stderr.String()))
		}
		return resp, fmt.Errorf("plugin %s returned invalid JSON: %w\nstdout: %s\nstderr: %s", p.binary, decErr, strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()))
	}
	if resp.Error != "" {
		return resp, errors.New(resp.Error)
	}
	if err != nil {
		return resp, err
	}
	if stderr.Len() > 0 {
		log.Printf("[plugin:%s] %s", p.name, strings.TrimSpace(stderr.String()))
	}
	return resp, nil
}

type ExternalProducer struct {
	plugin *compiledPlugin
}

func (p *ExternalProducer) Name() string { return p.plugin.name }

func (p *ExternalProducer) Match(url string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := p.plugin.call(ctx, pluginRequest{Method: "match", URL: url})
	if err != nil {
		log.Printf("[scriptmgr] producer %s match failed: %v", p.Name(), err)
		return false
	}
	return resp.Match
}

func (p *ExternalProducer) Produce(ctx context.Context, url string) (string, error) {
	resp, err := p.plugin.call(ctx, pluginRequest{Method: "produce", URL: url})
	if err != nil {
		return "", err
	}
	return resp.PDFPath, nil
}

type ExternalReceiver struct {
	plugin *compiledPlugin
}

func (r *ExternalReceiver) Name() string { return r.plugin.name }

func (r *ExternalReceiver) Receive(ctx context.Context, resources []sdk.Resource) error {
	_, err := r.plugin.call(ctx, pluginRequest{Method: "receive", Resources: resources})
	return err
}

type ExternalURLAspect struct {
	plugin *compiledPlugin
}

func (a *ExternalURLAspect) Name() string { return a.plugin.name }

func (a *ExternalURLAspect) Priority() int { return a.plugin.priority }

func (a *ExternalURLAspect) OnURL(ctx context.Context, url string) (string, error) {
	resp, err := a.plugin.call(ctx, pluginRequest{Method: "url", URL: url})
	if err != nil {
		return "", err
	}
	return resp.URL, nil
}

type ExternalReceiveAspect struct {
	plugin *compiledPlugin
}

func (a *ExternalReceiveAspect) Name() string { return a.plugin.name }

func (a *ExternalReceiveAspect) Priority() int { return a.plugin.priority }

func (a *ExternalReceiveAspect) BeforeReceive(ctx context.Context, resources []sdk.Resource) ([]sdk.Resource, error) {
	resp, err := a.plugin.call(ctx, pluginRequest{Method: "before_receive", Resources: resources})
	if err != nil {
		return nil, err
	}
	return resp.Resources, nil
}
