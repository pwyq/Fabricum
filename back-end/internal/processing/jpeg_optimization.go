package processing

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
)

const JPEGTranVersion = "3.1.3"

// JPEGOptimizationRequest optimizes an existing JPEG in place. All application
// and comment markers are preserved. Progressive requests progressive scans;
// otherwise the input's sequential/progressive mode is retained.
type JPEGOptimizationRequest struct {
	Path             string `json:"path"`
	EncoderDirectory string `json:"encoderDirectory,omitempty"`
	Progressive      bool   `json:"progressive,omitempty"`
}

type JPEGOptimizationResult struct {
	Path             string  `json:"path"`
	InputBytes       int     `json:"inputBytes"`
	OutputBytes      int     `json:"outputBytes"`
	SavingsBytes     int     `json:"savingsBytes"`
	SavingsPercent   float64 `json:"savingsPercent"`
	Format           string  `json:"format"`
	Progressive      bool    `json:"progressive"`
	Optimizer        string  `json:"optimizer"`
	OptimizerVersion string  `json:"optimizerVersion"`
	Changed          bool    `json:"changed"`
	Error            string  `json:"error,omitempty"`
}

// OptimizeJPEG changes entropy coding, never quality, subsampling, or DCT
// coefficients. A verified candidate replaces the source only if smaller.
func OptimizeJPEG(ctx context.Context, request JPEGOptimizationRequest) (JPEGOptimizationResult, error) {
	return optimizeJPEG(ctx, request, runJPEGOptimizer)
}

type jpegOptimizer func(context.Context, []byte, string, bool) ([]byte, string, error)

func optimizeJPEG(ctx context.Context, request JPEGOptimizationRequest, optimize jpegOptimizer) (JPEGOptimizationResult, error) {
	result := JPEGOptimizationResult{Path: request.Path, Optimizer: "jpegtran"}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if request.Path == "" {
		return result, fmt.Errorf("JPEG file path is required")
	}
	path, err := filepath.Abs(request.Path)
	if err != nil {
		return result, err
	}
	result.Path = path
	info, err := os.Lstat(path)
	if err != nil {
		return result, fmt.Errorf("inspect JPEG: %w", err)
	}
	if !info.Mode().IsRegular() {
		return result, fmt.Errorf("JPEG input must be a regular file, not a directory or symbolic link")
	}
	if info.Size() > maxInspectionFileBytes {
		return result, fmt.Errorf("JPEG exceeds %d byte limit", maxInspectionFileBytes)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return result, fmt.Errorf("read JPEG: %w", err)
	}
	result.InputBytes, result.OutputBytes = len(original), len(original)
	source, err := parseOptimizationJPEG(original)
	if err != nil {
		return result, fmt.Errorf("invalid JPEG input: %w", err)
	}
	result.Format, result.Progressive = "jpeg", source.progressive
	// libjpeg supplies default Huffman tables for abbreviated Motion JPEG
	// streams, even with -strict. Require a standalone image that Go's
	// independent decoder accepts before invoking the coefficient optimizer.
	if _, err := jpeg.Decode(bytes.NewReader(original)); err != nil {
		return result, fmt.Errorf("invalid or unsupported JPEG input: %w", err)
	}
	candidate, version, err := optimize(ctx, original, request.EncoderDirectory, request.Progressive || source.progressive)
	result.OptimizerVersion = version
	if err != nil {
		return result, err
	}
	optimized, err := parseOptimizationJPEG(candidate)
	if err != nil {
		return result, fmt.Errorf("invalid jpegtran output: %w", err)
	}
	if !sameJPEGMetadata(source, optimized) {
		return result, fmt.Errorf("jpegtran changed JPEG metadata")
	}
	if optimized.progressive != (request.Progressive || source.progressive) {
		return result, fmt.Errorf("jpegtran changed the requested scan mode")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if len(candidate) >= len(original) {
		return result, nil
	}
	if err := replaceOptimizedFile(ctx, path, original, candidate, info); err != nil {
		return result, fmt.Errorf("replace JPEG: %w", err)
	}
	result.OutputBytes = len(candidate)
	result.SavingsBytes = len(original) - len(candidate)
	result.SavingsPercent = float64(result.SavingsBytes) * 100 / float64(len(original))
	result.Changed, result.Progressive = true, optimized.progressive
	return result, nil
}
