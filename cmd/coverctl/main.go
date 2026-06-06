package main

import (
	"os"

	"go.klarlabs.de/coverctl/internal/cli"
)

func main() {
	code := cli.Run(os.Args, os.Stdout, os.Stderr, cli.BuildService(os.Stdout))
	os.Exit(code)
}
