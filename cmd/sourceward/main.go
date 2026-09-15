package main

import (
	"fmt"
	"os"

	"github.com/SourceWard/sourceward/internal/cli"
)

func main() {
	if err := cli.New().Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(cli.ExitCode(err))
	}
}
