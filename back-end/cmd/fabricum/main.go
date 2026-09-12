package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"fabricum/back-end"
)

func main() {
	config, err := fabricum.ParseConfig(os.Args[1:], os.Stdout)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "fabricum:", err)
		os.Exit(2)
	}
	if err := fabricum.Serve(config); err != nil {
		fmt.Fprintln(os.Stderr, "fabricum:", err)
		os.Exit(1)
	}
}
