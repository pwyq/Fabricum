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

const PNGOptimizationReceiptSchemaVersion = 1

type PNGOptimizationReceipt struct {
	SchemaVersion int                                `json:"schemaVersion"`
	Processor     string                             `json:"processor"`
	Files         []processing.PNGOptimizationResult `json:"files"`
}

// RunPNGOptimization emits per-file measurements, including errors, and then
// returns failure if any input failed. Each successful file commits separately.
func RunPNGOptimization(ctx context.Context, requests []processing.PNGOptimizationRequest, output io.Writer) error {
	if len(requests) == 0 {
		return fmt.Errorf("optimize-png requires at least one PNG file")
	}
	receipt := PNGOptimizationReceipt{SchemaVersion: PNGOptimizationReceiptSchemaVersion, Processor: "fabricum/" + editor.Version}
	var failures []error
	for _, request := range requests {
		result, err := processing.OptimizePNG(ctx, request)
		if err != nil {
			result.Error = err.Error()
			failures = append(failures, fmt.Errorf("%s: %w", request.Path, err))
		}
		receipt.Files = append(receipt.Files, result)
	}
	if err := json.NewEncoder(output).Encode(receipt); err != nil {
		return fmt.Errorf("write PNG optimization receipt: %w", err)
	}
	return errors.Join(failures...)
}

func RunPNGOptimizationArgs(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("optimize-png", flag.ContinueOnError)
	flags.SetOutput(output)
	directory := flags.String("encoder-directory", "", "directory containing pinned oxipng; defaults to embedded tools or PATH")
	strip := flags.Bool("strip-metadata", false, "remove text and timestamp chunks; preserve all rendering information")
	flags.Usage = func() {
		fmt.Fprintln(output, "Usage: fabricum optimize-png [--strip-metadata] [--encoder-directory directory] file.png [more.png ...]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	requests := make([]processing.PNGOptimizationRequest, 0, flags.NArg())
	for _, path := range flags.Args() {
		requests = append(requests, processing.PNGOptimizationRequest{Path: path, EncoderDirectory: *directory, StripMetadata: *strip})
	}
	return RunPNGOptimization(ctx, requests, output)
}
