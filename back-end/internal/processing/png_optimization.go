package processing

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
)

const OxipngVersion = "10.2.1"

// PNGOptimizationRequest optimizes an existing static PNG in place. Stripping
// removes only text and timestamp chunks; rendering information is retained.
type PNGOptimizationRequest struct {
	Path             string `json:"path"`
	EncoderDirectory string `json:"encoderDirectory,omitempty"`
	StripMetadata    bool   `json:"stripMetadata,omitempty"`
}

type PNGOptimizationResult struct {
	Path             string  `json:"path"`
	InputBytes       int     `json:"inputBytes"`
	OutputBytes      int     `json:"outputBytes"`
	SavingsBytes     int     `json:"savingsBytes"`
	SavingsPercent   float64 `json:"savingsPercent"`
	Format           string  `json:"format"`
	ColorType        string  `json:"colorType"`
	BitDepth         byte    `json:"bitDepth"`
	Optimizer        string  `json:"optimizer"`
	OptimizerVersion string  `json:"optimizerVersion"`
	Changed          bool    `json:"changed"`
	MetadataStripped bool    `json:"metadataStripped"`
	Error            string  `json:"error,omitempty"`
}

// OptimizePNG searches PNG compression without invoking the palette quantizer.
// Only a smaller candidate with identical pixels and rendering chunks is written.
func OptimizePNG(ctx context.Context, request PNGOptimizationRequest) (PNGOptimizationResult, error) {
	return optimizePNG(ctx, request, runPNGOptimizer)
}

type pngOptimizer func(context.Context, []byte, string) ([]byte, string, error)

func optimizePNG(ctx context.Context, request PNGOptimizationRequest, optimize pngOptimizer) (PNGOptimizationResult, error) {
	result := PNGOptimizationResult{Path: request.Path, Optimizer: "oxipng"}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if request.Path == "" {
		return result, fmt.Errorf("PNG file path is required")
	}
	path, err := filepath.Abs(request.Path)
	if err != nil {
		return result, err
	}
	result.Path = path
	info, err := os.Lstat(path)
	if err != nil {
		return result, fmt.Errorf("inspect PNG: %w", err)
	}
	if !info.Mode().IsRegular() {
		return result, fmt.Errorf("PNG input must be a regular file, not a directory or symbolic link")
	}
	if info.Size() > maxInspectionFileBytes {
		return result, fmt.Errorf("PNG exceeds %d byte limit", maxInspectionFileBytes)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return result, fmt.Errorf("read PNG: %w", err)
	}
	result.InputBytes, result.OutputBytes = len(original), len(original)
	source, err := decodeOptimizationPNG(original)
	if err != nil {
		return result, fmt.Errorf("invalid PNG input: %w", err)
	}
	result.Format = "png"
	result.BitDepth = source.chunks[0].data[8]
	result.ColorType = pngColorType(source.chunks[0].data[9])
	candidate, version, err := optimize(ctx, original, request.EncoderDirectory)
	result.OptimizerVersion = version
	if err != nil {
		return result, err
	}
	optimized, err := decodeOptimizationPNG(candidate)
	if err != nil {
		return result, fmt.Errorf("invalid oxipng output: %w", err)
	}
	if !bytes.Equal(source.chunks[0].data, optimized.chunks[0].data) {
		return result, fmt.Errorf("oxipng changed PNG storage or dimensions")
	}
	if err := samePNGPixelValues(ctx, source.pixels, optimized.pixels); err != nil {
		return result, fmt.Errorf("oxipng violated the lossless contract: %w", err)
	}
	candidate, stripped := rebuildOptimizedPNG(source.chunks, optimized.chunks, request.StripMetadata)
	// Validate the assembled file too, including original palette/transparency.
	rebuilt, err := decodeOptimizationPNG(candidate)
	if err != nil {
		return result, fmt.Errorf("invalid assembled PNG: %w", err)
	}
	if err := samePNGPixelValues(ctx, source.pixels, rebuilt.pixels); err != nil {
		return result, fmt.Errorf("assembled PNG violated the lossless contract: %w", err)
	}
	if len(candidate) >= len(original) {
		return result, nil
	}
	if err := replaceOptimizedPNG(ctx, path, original, candidate, info); err != nil {
		return result, fmt.Errorf("replace PNG: %w", err)
	}
	result.OutputBytes = len(candidate)
	result.SavingsBytes = len(original) - len(candidate)
	result.SavingsPercent = float64(result.SavingsBytes) * 100 / float64(len(original))
	result.Changed, result.MetadataStripped = true, stripped
	return result, nil
}

func pngColorType(kind byte) string {
	switch kind {
	case 0:
		return "grayscale"
	case 2:
		return "rgb"
	case 3:
		return "indexed"
	case 4:
		return "grayscale-alpha"
	case 6:
		return "rgba"
	default:
		return "unknown"
	}
}
