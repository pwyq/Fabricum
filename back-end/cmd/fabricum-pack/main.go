package main

import (
	"flag"
	"fmt"
	"os"

	"fabricum/back-end/internal/releasebinary"
)

func main() {
	options := releasebinary.Options{}
	flag.StringVar(&options.Input, "input", "", "Fabricum executable to package")
	flag.StringVar(&options.NativeDirectory, "native-directory", "", "directory containing native tools")
	flag.StringVar(&options.Output, "output", "", "standalone release executable")
	flag.StringVar(&options.Root, "root", ".", "repository root containing license metadata")
	flag.StringVar(&options.Platform, "platform", "", "windows-x64 or linux-x64")
	flag.Parse()
	if options.Input == "" || options.NativeDirectory == "" || options.Output == "" || options.Platform == "" {
		fmt.Fprintln(os.Stderr, "fabricum-pack: input, native-directory, output, and platform are required")
		os.Exit(2)
	}
	if err := releasebinary.Build(options); err != nil {
		fmt.Fprintln(os.Stderr, "fabricum-pack:", err)
		os.Exit(1)
	}
}
