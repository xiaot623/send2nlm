package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"send2nlm/core"
)

func runStop(args []string) error {
	fs := newFlagSet("stop")
	dev := fs.Bool("dev", false, "use dev_assets")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := core.NewRuntimeConfig(*dev)
	data, err := os.ReadFile(cfg.PIDFile())
	if err != nil {
		return fmt.Errorf("cannot read pid file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return err
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}
