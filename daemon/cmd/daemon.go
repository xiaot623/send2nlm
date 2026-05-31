package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"send2nlm/core"
	"send2nlm/producer"
	"send2nlm/receiver"
	"send2nlm/resources"
	"send2nlm/scriptmgr"
	"send2nlm/sdk"
	"send2nlm/server"
	"send2nlm/store"
)

func runDaemon(args []string) error {
	fs := newFlagSet("daemon")
	port := fs.Int("port", core.DefaultPort, "daemon port")
	dev := fs.Bool("dev", false, "use dev_assets")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := core.NewRuntimeConfig(*dev)
	if err := cfg.Ensure(); err != nil {
		return err
	}
	if *port != core.DefaultPort {
		cfg.Port = *port
	}

	db, err := store.Open(cfg.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	// ── Plugin system bootstrap ───────────────────────────────────────
	bootstrapPluginSystem(cfg)

	loader := scriptmgr.NewLoader(cfg.ConfigDir, cfg.PluginCacheDir())

	// Producer registry
	producerReg := scriptmgr.NewProducerRegistry()
	producerReg.RegisterBuiltin(&producer.DefaultProducer{})
	scriptmgr.LoadProducerDir(loader, producerReg, cfg.ProducerDir())
	_ = scriptmgr.WatchProducerDir(loader, producerReg, cfg.ProducerDir())

	// Receiver registry
	receiverReg := scriptmgr.NewReceiverRegistry()
	receiverReg.RegisterBuiltin(receiver.NewDownloadReceiver())
	scriptmgr.LoadReceiverDir(loader, receiverReg, cfg.ReceiverDir())
	_ = scriptmgr.WatchReceiverDir(loader, receiverReg, cfg.ReceiverDir())

	log.Printf("[daemon] external plugin system initialized")

	app := server.NewApp(cfg, db, version, producerReg, receiverReg)

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.Port))
	if err != nil {
		if cfg.Port == core.DefaultPort {
			return fmt.Errorf("default port %d unavailable, retry with --port: %w", cfg.Port, err)
		}
		return err
	}
	defer ln.Close()

	if err := os.WriteFile(cfg.PortFile(), []byte(fmt.Sprintf("%d\n", cfg.Port)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(cfg.PIDFile(), []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644); err != nil {
		return err
	}
	defer os.Remove(cfg.PIDFile())

	srv := &http.Server{
		Addr:              ln.Addr().String(),
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// bootstrapPluginSystem sets the SDK config directory and installs embedded
// resource scripts (lark.go, telegram.go) into the user config dir on first run.
func bootstrapPluginSystem(cfg core.RuntimeConfig) {
	sdk.SetConfigDir(cfg.ConfigDir)

	// Install embedded producer scripts
	producerDir := cfg.ProducerDir()
	if err := os.MkdirAll(producerDir, 0o755); err != nil {
		log.Printf("[daemon] cannot create producer dir: %v", err)
		return
	}
	installEmbedded(producerDir, "lark.go", "producer/lark.go")

	// Install embedded receiver scripts
	receiverDir := cfg.ReceiverDir()
	if err := os.MkdirAll(receiverDir, 0o755); err != nil {
		log.Printf("[daemon] cannot create receiver dir: %v", err)
		return
	}
	installEmbedded(receiverDir, "telegram.go", "receiver/telegram.go")
}

// installEmbedded copies an embedded resource file to the target directory
// if it doesn't already exist.
func installEmbedded(dir, filename, embedPath string) {
	dst := filepath.Join(dir, filename)
	if _, err := os.Stat(dst); err == nil {
		return // already exists, don't overwrite user modifications
	}
	data, err := resources.Files.ReadFile(embedPath)
	if err != nil {
		log.Printf("[daemon] embedded resource %s not found: %v", embedPath, err)
		return
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		log.Printf("[daemon] cannot install %s: %v", filename, err)
		return
	}
	log.Printf("[daemon] installed %s → %s", filename, dst)
}
