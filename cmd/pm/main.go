package main

import (
	"fmt"
	"os"

	"github.com/pommel-dev/pommel/internal/cli"
)

// Set at build time via ldflags
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Set version info
	cli.Version = version
	cli.BuildCommit = commit
	cli.BuildDate = date

	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
