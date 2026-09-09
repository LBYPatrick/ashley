// Command ash is the standalone Go implementation of Ashley.
package main

import (
	"errors"
	"github.com/LBYPatrick/ashley/internal/cli"
	"github.com/LBYPatrick/ashley/internal/execution"
	"os"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		cli.WriteError(os.Stderr, err)
		var exit execution.ExitError
		if errors.As(err, &exit) {
			os.Exit(exit.Code)
		}
		os.Exit(1)
	}
}
