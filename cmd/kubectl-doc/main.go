package main

import (
	"os"

	"github.com/sttts/kubectl-doc/internal/cli"
)

func main() {
	cmd := cli.NewCommand(os.Stdout, os.Stderr)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
