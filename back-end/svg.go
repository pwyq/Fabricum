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

const SVGOptimizerVersion = processing.SVGOptimizerVersion
const SVGOptimizationReceiptSchemaVersion = cli.SVGOptimizationReceiptSchemaVersion

// OptimizeSVG optimizes a static SVG in place without fetching resources or
// rounding geometry. The validated candidate must be smaller than the source.
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
