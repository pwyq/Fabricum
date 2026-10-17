package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"fabricum/back-end/internal/editor"
	"fabricum/back-end/internal/processing"
)

const JPEGOptimizationReceiptSchemaVersion = 1

type JPEGOptimizationReceipt struct {
	SchemaVersion int                                 `json:"schemaVersion"`
	Processor     string                              `json:"processor"`
	Files         []processing.JPEGOptimizationResult `json:"files"`
}

// RunJPEGOptimization reports every file; each successful replacement commits
// independently and any failure makes the batch fail after writing the receipt.
func RunJPEGOptimization(ctx context.Context, requests []processing.JPEGOptimizationRequest, output io.Writer) error {
	if len(requests) == 0 {
		return fmt.Errorf("optimize-jpeg requires at least one JPEG file")
	}
	receipt := JPEGOptimizationReceipt{SchemaVersion: JPEGOptimizationReceiptSchemaVersion, Processor: "fabricum/" + editor.Version}
	var failures []error
	for _, request := range requests {
		result, err := processing.OptimizeJPEG(ctx, request)
		if err != nil {
			result.Error = err.Error()
			failures = append(failures, fmt.Errorf("%s: %w", request.Path, err))
		}
		receipt.Files = append(receipt.Files, result)
	}
	if err := json.NewEncoder(output).Encode(receipt); err != nil {
		return fmt.Errorf("write JPEG optimization receipt: %w", err)
	}
	return errors.Join(failures...)
}

func RunJPEGOptimizationArgs(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("optimize-jpeg", flag.ContinueOnError)
	flags.SetOutput(output)
	directory := flags.String("encoder-directory", "", "directory containing pinned jpegtran; defaults to embedded tools or PATH")
	progressive := flags.Bool("progressive", false, "use progressive scans if smaller; otherwise preserve the original file")
	flags.Usage = func() {
		fmt.Fprintln(output, "Usage: fabricum optimize-jpeg [--progressive] [--encoder-directory directory] file.jpg [more.jpg ...]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	requests := make([]processing.JPEGOptimizationRequest, 0, flags.NArg())
	for _, path := range flags.Args() {
		requests = append(requests, processing.JPEGOptimizationRequest{Path: path, EncoderDirectory: *directory, Progressive: *progressive})
	}
	return RunJPEGOptimization(ctx, requests, output)
}
