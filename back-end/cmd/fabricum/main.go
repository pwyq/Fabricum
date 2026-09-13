package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"fabricum/back-end"
)

func main() {
	args := os.Args[1:]
	if requestPath, ok := transformInvocation(args); ok {
		if err := fabricum.TransformFile(context.Background(), requestPath, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "fabricum:", err)
			os.Exit(1)
		}
		return
	}
	if requestPath, ok := textureSetInvocation(args); ok {
		if err := fabricum.TextureSetFile(context.Background(), requestPath, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "fabricum:", err)
			os.Exit(1)
		}
		return
	}
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

func transformInvocation(args []string) (string, bool) {
	if len(args) > 0 && (args[0] == "transform" || args[0] == "--transform") {
		if len(args) != 2 {
			return "", true
		}
		return args[1], true
	}
	return "", false
}

func textureSetInvocation(args []string) (string, bool) {
	if len(args) > 0 && (args[0] == "texture-set" || args[0] == "texture" || args[0] == "ktx2" || args[0] == "--texture-set") {
		if len(args) != 2 {
			return "", true
		}
		return args[1], true
	}
	return "", false
}
