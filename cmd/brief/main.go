// Command brief manages feature specifications as files in your
// repository. See internal/cli for the command surface.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/koblas/brief/internal/cli"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "brief:", err)
		os.Exit(1)
	}

	err = cli.Run(context.Background(), wd, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)

	os.Exit(cli.ExitCode(err))
}
