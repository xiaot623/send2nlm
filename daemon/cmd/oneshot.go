package cmd

import (
	"encoding/json"
	"fmt"

	"send2nlm/core"
	"send2nlm/store"
)

func runSend(args []string) error {
	fs := newFlagSet("send")
	notebookID := fs.String("notebook", "", "target notebook id")
	url := fs.String("url", "", "page url")
	tasksCSV := fs.String("tasks", "", "comma separated tasks")
	sourcesCSV := fs.String("sources", "", "comma separated source ids")
	dev := fs.Bool("dev", false, "use dev_assets")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *notebookID == "" {
		return fmt.Errorf("send requires --notebook")
	}

	cfg := core.NewRuntimeConfig(*dev)
	if err := cfg.Ensure(); err != nil {
		return err
	}

	db, err := store.Open(cfg.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()

	job := core.NewJob(*notebookID, "", *url, core.ParseTaskList(*tasksCSV), core.ParseTaskList(*sourcesCSV))
	if err := db.CreateJob(job); err != nil {
		return err
	}

	out, _ := json.MarshalIndent(job, "", "  ")
	fmt.Println(string(out))
	return nil
}
