package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"fabricum/back-end"
	"fabricum/back-end/internal/bundledtools"
	"fabricum/back-end/internal/compatibility"
)

func main() {
	args := os.Args[1:]
	if noticesInvocation(args) {
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "fabricum: third-party-notices does not accept arguments")
			os.Exit(2)
		}
		notices, err := bundledtools.Notices()
		if err != nil {
			fmt.Fprintln(os.Stderr, "fabricum:", err)
			os.Exit(1)
		}
		fmt.Fprint(os.Stdout, notices)
		return
	}
	if compatibilityInvocation(args) {
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "fabricum: compatibility-check does not accept arguments")
			os.Exit(2)
		}
		if err := compatibility.Run(context.Background(), os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "fabricum:", err)
			os.Exit(1)
		}
		return
	}
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
	if requestPath, ok := modelOptimizationInvocation(args); ok {
		if err := fabricum.ModelOptimizationFile(context.Background(), requestPath, os.Stdout); err != nil {
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

func noticesInvocation(args []string) bool {
	return len(args) > 0 && (args[0] == "third-party-notices" || args[0] == "licenses")
}

func compatibilityInvocation(args []string) bool {
	return len(args) > 0 && (args[0] == "compatibility-check" || args[0] == "--compatibility-check")
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

func modelOptimizationInvocation(args []string) (string, bool) {
	if len(args) > 0 && (args[0] == "model" || args[0] == "optimize-model" || args[0] == "gltfpack" || args[0] == "--model") {
		if len(args) != 2 {
			return "", true
		}
		return args[1], true
	}
	return "", false
}
