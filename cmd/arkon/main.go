package main

import (
	"context"
	"os"

	"github.com/adrianfinantyo/arkon/internal/cli"
	"github.com/adrianfinantyo/arkon/internal/env"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	hostEnv := env.NewRealEnv()
	runner := cli.NewRunner(hostEnv, os.Stdout, os.Stderr, cli.VersionInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	})

	exitCode := runner.Run(context.Background(), os.Args[1:])
	os.Exit(exitCode)
}
