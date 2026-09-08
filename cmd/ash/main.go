// Command ash is the experimental standalone Go implementation of Ashley.
package main

import (
	"errors"
	"fmt"
	"github.com/LBYPatrick/ashley/internal/cli"
	"github.com/LBYPatrick/ashley/internal/execution"
	"os"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		var exit execution.ExitError
		if errors.As(err, &exit) {
			os.Exit(exit.Code)
		}
		os.Exit(1)
	}
}
