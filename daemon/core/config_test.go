package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeConfigAspectDirs(t *testing.T) {
	t.Setenv("SEND2NLM_DEV", "")

	cfg := NewRuntimeConfig(false)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	if got, want := cfg.AspectDir(), filepath.Join(home, ".send2nlm", "aspect"); got != want {
		t.Fatalf("AspectDir() = %q, want %q", got, want)
	}
	if got, want := cfg.URLAspectDir(), filepath.Join(home, ".send2nlm", "aspect", "url"); got != want {
		t.Fatalf("URLAspectDir() = %q, want %q", got, want)
	}
	if got, want := cfg.ReceiveAspectDir(), filepath.Join(home, ".send2nlm", "aspect", "receive"); got != want {
		t.Fatalf("ReceiveAspectDir() = %q, want %q", got, want)
	}
}

func TestRuntimeConfigDevAspectDirs(t *testing.T) {
	t.Setenv("SEND2NLM_DEV", "")

	cfg := NewRuntimeConfig(true)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	wantBase := filepath.Join(filepath.Dir(cwd), "dev_assets", "aspect")

	if got := cfg.AspectDir(); got != wantBase {
		t.Fatalf("AspectDir() = %q, want %q", got, wantBase)
	}
	if got, want := cfg.URLAspectDir(), filepath.Join(wantBase, "url"); got != want {
		t.Fatalf("URLAspectDir() = %q, want %q", got, want)
	}
	if got, want := cfg.ReceiveAspectDir(), filepath.Join(wantBase, "receive"); got != want {
		t.Fatalf("ReceiveAspectDir() = %q, want %q", got, want)
	}
}

func TestRuntimeConfigEnvDevAspectDirs(t *testing.T) {
	t.Setenv("SEND2NLM_DEV", "1")

	cfg := NewRuntimeConfig(false)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(cwd), "dev_assets", "aspect")

	if got := cfg.AspectDir(); got != want {
		t.Fatalf("AspectDir() = %q, want %q", got, want)
	}
}
