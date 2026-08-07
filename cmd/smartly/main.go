package main

import (
	"fmt"
	"os"

	"github.com/rizwanreza/smartly-cli/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "smartly:", err)
		os.Exit(1)
	}
}
