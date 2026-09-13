package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"fabricum/back-end"
)

func main() {
	args := os.Args[1:]
	if paths, ok := inspectionInvocation(args); ok {
		if err := fabricum.Inspect(paths, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "fabricum:", err)
			os.Exit(1)
		}
		return
	}
	config, err := fabricum.ParseConfig(args, os.Stdout)
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

func inspectionInvocation(args []string) ([]string, bool) {
	if len(args) > 0 && (args[0] == "inspect" || args[0] == "--inspect") {
		return args[1:], true
	}
	return nil, false
}
