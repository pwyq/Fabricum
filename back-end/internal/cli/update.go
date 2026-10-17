package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"

	"fabricum/back-end/internal/editor"
	"fabricum/back-end/internal/updater"
)

// RunUpdateArgs checks for and installs a compatible stable release, writing
// exactly one JSON result on success or failure. Help is plain text.
func RunUpdateArgs(ctx context.Context, args []string, output io.Writer) error {
	return runUpdateArgs(ctx, args, output, nil)
}

func runUpdateArgs(ctx context.Context, args []string, output io.Writer, client *http.Client) error {
	flags := flag.NewFlagSet("update", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() {}
	options := updater.Options{CurrentVersion: editor.Version}
	flags.StringVar(&options.InstallDirectory, "install-directory", "", "managed install root (defaults to the user cache)")
	flags.StringVar(&options.MinVersion, "min-version", "", "inclusive minimum stable version, e.g. 0.2.0")
	flags.StringVar(&options.MaxVersion, "max-version", "", "exclusive maximum stable version, e.g. 1.0.0")
	flags.DurationVar(&options.Timeout, "timeout", updater.DefaultTimeout, "total deadline for discovery and installation, at most 10m")
	err := flags.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(output, "Usage: fabricum update [--install-directory path] [--min-version X.Y.Z] [--max-version X.Y.Z] [--timeout 2m]")
		fmt.Fprintln(output, "Check GitHub and install the newest compatible stable release; print a JSON result.")
		flags.SetOutput(output)
		flags.PrintDefaults()
		return flag.ErrHelp
	}
	if err == nil && flags.NArg() != 0 {
		err = fmt.Errorf("update does not accept positional arguments")
	}
	if err == nil && options.Timeout <= 0 {
		err = fmt.Errorf("timeout must be greater than zero")
	}
	result := updater.Result{SchemaVersion: 1, Status: "failed"}
	if err == nil {
		result, err = updater.Update(ctx, options, client)
	}
	if err != nil {
		result.Error = err.Error()
	}
	if writeErr := json.NewEncoder(output).Encode(result); writeErr != nil {
		return errors.Join(err, fmt.Errorf("write update result: %w", writeErr))
	}
	return err
}
