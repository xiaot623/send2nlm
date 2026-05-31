package cmd

import (
	"fmt"

	"send2nlm/core"
)

func runConfig(args []string) error {
	fs := newFlagSet("config")
	pathOnly := fs.Bool("path", false, "print config directory path")
	dev := fs.Bool("dev", false, "use dev_assets")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := core.NewRuntimeConfig(*dev)
	if *pathOnly {
		fmt.Println(cfg.ConfigDir)
		return nil
	}

	fmt.Printf("config_dir=%s\n", cfg.ConfigDir)
	fmt.Printf("db_path=%s\n", cfg.DBPath())
	return nil
}
