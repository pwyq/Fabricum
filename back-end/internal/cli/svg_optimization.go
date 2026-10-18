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

const SVGOptimizationReceiptSchemaVersion = 1

type SVGOptimizationReceipt struct {
	SchemaVersion int                                `json:"schemaVersion"`
	Processor     string                             `json:"processor"`
	Files         []processing.SVGOptimizationResult `json:"files"`
}

// RunSVGOptimization writes a result for every file, including failures.
// Each successful replacement is independent; any failure fails the batch.
func RunSVGOptimization(ctx context.Context, requests []processing.SVGOptimizationRequest, output io.Writer) error {
	if len(requests) == 0 {
		return fmt.Errorf("optimize-svg requires at least one SVG file")
	}
	receipt := SVGOptimizationReceipt{SchemaVersion: SVGOptimizationReceiptSchemaVersion, Processor: "fabricum/" + editor.Version}
	var failures []error
	for _, request := range requests {
		result, err := processing.OptimizeSVG(ctx, request)
		if err != nil {
			result.Error = err.Error()
			failures = append(failures, fmt.Errorf("%s: %w", request.Path, err))
		}
		receipt.Files = append(receipt.Files, result)
	}
	if err := json.NewEncoder(output).Encode(receipt); err != nil {
		return fmt.Errorf("write SVG optimization receipt: %w", err)
	}
	return errors.Join(failures...)
}

func RunSVGOptimizationArgs(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("optimize-svg", flag.ContinueOnError)
	flags.SetOutput(output)
	var options processing.SVGOptimizationOptions
	var destination string
	var precision int
	flags.BoolVar(&options.Icon, "icon", false, "clean editor data and simple inline presentation styles; minify paths")
	flags.IntVar(&precision, "precision", -1, "round path coordinates/stroke widths to 0..8 decimal places and viewBox values to integers (default: unchanged)")
	flags.BoolVar(&options.SelectiveRounding, "selective-rounding", false, "round terminal 1/9 at thousandths, then hundredths, in path coordinates")
	flags.BoolVar(&options.CurrentColor, "current-color", false, "use currentColor for single-color icons")
	flags.StringVar(&destination, "output", "", "write one input to a new destination, preserving the source")
	flags.Usage = func() {
		fmt.Fprintln(output, "Usage: fabricum optimize-svg [options] [--] file.svg [more.svg ...]")
		fmt.Fprintln(output, "Optimize static SVGs offline in place; retain originals unless smaller.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	precisionSet := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "precision" {
			precisionSet = true
		}
	})
	if precisionSet {
		if precision < 0 || precision > 8 {
			return fmt.Errorf("SVG precision must be between 0 and 8")
		}
		options.Precision = &precision
	}
	if destination != "" && flags.NArg() != 1 {
		return fmt.Errorf("--output requires exactly one SVG input")
	}
	requests := make([]processing.SVGOptimizationRequest, 0, flags.NArg())
	for _, path := range flags.Args() {
		requests = append(requests, processing.SVGOptimizationRequest{Path: path, OutputPath: destination, SVGOptimizationOptions: options})
	}
	return RunSVGOptimization(ctx, requests, output)
}
