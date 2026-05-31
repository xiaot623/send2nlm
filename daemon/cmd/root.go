package cmd

import (
	"errors"
	"flag"
	"fmt"
)

func Run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}

	switch args[0] {
	case "daemon":
		return runDaemon(args[1:])
	case "send":
		return runSend(args[1:])
	case "stop":
		return runStop(args[1:])
	case "config":
		return runConfig(args[1:])
	case "version":
		fmt.Println(version)
		return nil
	default:
		return usageError()
	}
}

func usageError() error {
	return errors.New("usage: send2nlm <daemon|send|stop|config|version>")
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(flag.CommandLine.Output())
	return fs
}
