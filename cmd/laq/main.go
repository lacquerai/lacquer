package main

import (
	"os"

	"github.com/lacquer/lacquer/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
