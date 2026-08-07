package main

import (
	"os"

	"github.com/rizwanreza/smartly-cli/internal/cli"
)

func main() {
	// Execute has already reported the failure in smartly's error
	// vocabulary; the error is only used here to pick an exit code.
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
