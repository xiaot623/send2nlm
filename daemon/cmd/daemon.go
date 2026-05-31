package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"send2nlm/core"
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

	app := server.NewApp(cfg, db, version)

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
