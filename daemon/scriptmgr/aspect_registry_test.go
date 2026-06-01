package scriptmgr

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"send2nlm/sdk"
)

type testURLAspect struct {
	name     string
	priority int
	fn       func(string) (string, error)
}

func (a testURLAspect) Name() string  { return a.name }
func (a testURLAspect) Priority() int { return a.priority }
func (a testURLAspect) OnURL(_ context.Context, url string) (string, error) {
	return a.fn(url)
}

type testReceiveAspect struct {
	name     string
	priority int
	fn       func([]sdk.Resource) ([]sdk.Resource, error)
}

func (a testReceiveAspect) Name() string  { return a.name }
func (a testReceiveAspect) Priority() int { return a.priority }
func (a testReceiveAspect) BeforeReceive(_ context.Context, resources []sdk.Resource) ([]sdk.Resource, error) {
	return a.fn(resources)
}

func TestURLAspectRegistryAppliesPriorityOrder(t *testing.T) {
	reg := NewURLAspectRegistry()
	reg.SetScripts([]sdk.URLAspect{
		testURLAspect{name: "z-same", priority: 10, fn: suffix("-z")},
		testURLAspect{name: "high", priority: 100, fn: suffix("-high")},
		testURLAspect{name: "a-same", priority: 10, fn: suffix("-a")},
	})

	got, err := reg.Apply(context.Background(), "url")
	if err != nil {
		t.Fatal(err)
	}
	if want := "url-high-a-z"; got != want {
		t.Fatalf("Apply() = %q, want %q", got, want)
	}
}

func TestURLAspectRegistryStopsOnError(t *testing.T) {
	wantErr := errors.New("boom")
	reg := NewURLAspectRegistry()
	reg.SetScripts([]sdk.URLAspect{
		testURLAspect{name: "first", priority: 10, fn: suffix("-first")},
		testURLAspect{name: "bad", priority: 9, fn: func(string) (string, error) { return "", wantErr }},
		testURLAspect{name: "last", priority: 8, fn: suffix("-last")},
	})

	_, err := reg.Apply(context.Background(), "url")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Apply() error = %v, want %v", err, wantErr)
	}
}

func TestReceiveAspectRegistryAppliesPriorityOrder(t *testing.T) {
	reg := NewReceiveAspectRegistry()
	reg.SetScripts([]sdk.ReceiveAspect{
		testReceiveAspect{name: "low", priority: 1, fn: addResource("low")},
		testReceiveAspect{name: "high", priority: 2, fn: addResource("high")},
	})

	got, err := reg.Apply(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].TaskType != "high" || got[1].TaskType != "low" {
		t.Fatalf("Apply() resources = %#v, want high then low", got)
	}
}

func TestAspectPluginLoadingAndExecution(t *testing.T) {
	configDir := t.TempDir()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	urlDir := filepath.Join(t.TempDir(), "url")
	receiveDir := filepath.Join(t.TempDir(), "receive")
	if err := os.MkdirAll(urlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(receiveDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(urlDir, "url.go"), `package aspect

import (
	"context"
	"send2nlm/sdk"
)

type A struct{}
func (A) Name() string { return "url-script" }
func (A) Priority() int { return 7 }
func (A) OnURL(ctx context.Context, url string) (string, error) { return url + "?aspect=1", nil }
var URLAspect sdk.URLAspect = A{}
`)
	writeTestFile(t, filepath.Join(receiveDir, "receive.go"), `package aspect

import (
	"context"
	"send2nlm/sdk"
)

type A struct{}
func (A) Name() string { return "receive-script" }
func (A) Priority() int { return 3 }
func (A) BeforeReceive(ctx context.Context, resources []sdk.Resource) ([]sdk.Resource, error) {
	resources[0].DeliveryName = resources[0].DeliveryName + ".aspect"
	return resources, nil
}
var ReceiveAspect sdk.ReceiveAspect = A{}
`)

	loader := NewLoader(configDir, cacheDir)
	urlReg := NewURLAspectRegistry()
	LoadURLAspectDir(loader, urlReg, urlDir)
	gotURL, err := urlReg.Apply(context.Background(), "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://example.com?aspect=1"; gotURL != want {
		t.Fatalf("URL aspect output = %q, want %q", gotURL, want)
	}

	receiveReg := NewReceiveAspectRegistry()
	LoadReceiveAspectDir(loader, receiveReg, receiveDir)
	resources, err := receiveReg.Apply(context.Background(), []sdk.Resource{{DeliveryName: "file.pdf"}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := resources[0].DeliveryName, "file.pdf.aspect"; got != want {
		t.Fatalf("receive aspect output = %q, want %q", got, want)
	}
}

func TestEmptyAspectDirs(t *testing.T) {
	loader := NewLoader(t.TempDir(), filepath.Join(t.TempDir(), "cache"))
	urlDir := t.TempDir()
	receiveDir := t.TempDir()

	urlReg := NewURLAspectRegistry()
	LoadURLAspectDir(loader, urlReg, urlDir)
	gotURL, err := urlReg.Apply(context.Background(), "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if gotURL != "https://example.com" {
		t.Fatalf("empty URL aspect dir changed URL to %q", gotURL)
	}

	receiveReg := NewReceiveAspectRegistry()
	LoadReceiveAspectDir(loader, receiveReg, receiveDir)
	in := []sdk.Resource{{DeliveryName: "file.pdf"}}
	gotResources, err := receiveReg.Apply(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotResources) != 1 || gotResources[0].DeliveryName != "file.pdf" {
		t.Fatalf("empty receive aspect dir changed resources to %#v", gotResources)
	}
}

func suffix(s string) func(string) (string, error) {
	return func(in string) (string, error) {
		return in + s, nil
	}
}

func addResource(taskType string) func([]sdk.Resource) ([]sdk.Resource, error) {
	return func(in []sdk.Resource) ([]sdk.Resource, error) {
		return append(in, sdk.Resource{TaskType: taskType}), nil
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
