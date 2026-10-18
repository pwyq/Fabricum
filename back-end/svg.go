package fabricum

import (
	"context"
	"io"

	"fabricum/back-end/internal/cli"
	"fabricum/back-end/internal/processing"
)

type SVGOptimizationRequest = processing.SVGOptimizationRequest
type SVGOptimizationResult = processing.SVGOptimizationResult
type SVGOptimizationReceipt = cli.SVGOptimizationReceipt
type SVGOptimizationOptions = processing.SVGOptimizationOptions
type SVGPathError = processing.SVGPathError

const SVGOptimizerVersion = processing.SVGOptimizerVersion
const SVGOptimizationReceiptSchemaVersion = cli.SVGOptimizationReceiptSchemaVersion

// OptimizeSVG optimizes a static SVG offline, with optional icon preparation,
// numeric rounding, and a separate destination. The default preserves precision.
func OptimizeSVG(ctx context.Context, request SVGOptimizationRequest) (SVGOptimizationResult, error) {
	return processing.OptimizeSVG(ctx, request)
}

// OptimizeSVGFiles writes a versioned JSON receipt, including file failures.
func OptimizeSVGFiles(ctx context.Context, requests []SVGOptimizationRequest, output io.Writer) error {
	return cli.RunSVGOptimization(ctx, requests, output)
}

func OptimizeSVGArgs(ctx context.Context, args []string, output io.Writer) error {
	return cli.RunSVGOptimizationArgs(ctx, args, output)
}
