package main

import (
	"os"

	"github.com/missdeer/notebooklm-client/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
