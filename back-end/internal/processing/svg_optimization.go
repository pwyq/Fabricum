package processing

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SVGOptimizerVersion identifies the in-tree, Apache-2.0 static SVG profile.
// It uses only Go's standard library and has no bundled third-party packages.
const SVGOptimizerVersion = "1.0.0"

type SVGOptimizationRequest struct {
	Path string `json:"path"`
}

type SVGOptimizationResult struct {
	Path             string   `json:"path"`
	InputBytes       int      `json:"inputBytes"`
	OutputBytes      int      `json:"outputBytes"`
	SavingsBytes     int      `json:"savingsBytes"`
	SavingsPercent   float64  `json:"savingsPercent"`
	Format           string   `json:"format"`
	Optimizer        string   `json:"optimizer"`
	OptimizerVersion string   `json:"optimizerVersion"`
	Changed          bool     `json:"changed"`
	Diagnostics      []string `json:"diagnostics,omitempty"`
	Error            string   `json:"error,omitempty"`
}

// OptimizeSVG optimizes one static SVG in place. It never fetches resources,
// evaluates code, rounds numbers, or replaces an equal/larger original.
func OptimizeSVG(ctx context.Context, request SVGOptimizationRequest) (SVGOptimizationResult, error) {
	result := SVGOptimizationResult{Path: request.Path, Optimizer: "fabricum-svg", OptimizerVersion: SVGOptimizerVersion}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if request.Path == "" {
		return result, fmt.Errorf("SVG file path is required")
	}
	path, err := filepath.Abs(request.Path)
	if err != nil {
		return result, err
	}
	result.Path = path
	info, err := os.Lstat(path)
	if err != nil {
		return result, fmt.Errorf("inspect SVG: %w", err)
	}
	if !info.Mode().IsRegular() {
		return result, fmt.Errorf("SVG input must be a regular file, not a directory or symbolic link")
	}
	if info.Size() > maxSVGBytes {
		return result, fmt.Errorf("SVG exceeds %d byte limit", maxSVGBytes)
	}
	file, err := os.Open(path)
	if err != nil {
		return result, err
	}
	original, err := io.ReadAll(io.LimitReader(file, maxSVGBytes+1))
	closeErr := file.Close()
	if err != nil {
		return result, err
	}
	if closeErr != nil {
		return result, closeErr
	}
	result.InputBytes, result.OutputBytes = len(original), len(original)
	if len(original) > maxSVGBytes {
		return result, fmt.Errorf("SVG exceeds %d byte limit", maxSVGBytes)
	}
	candidate, diagnostics, err := optimizeSVGDocument(ctx, original)
	result.Diagnostics = diagnostics
	if err != nil {
		return result, err
	}
	result.Format = "svg"
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if len(candidate) >= len(original) {
		return result, nil
	}
	if err := replaceOptimizedFile(ctx, path, original, candidate, info); err != nil {
		return result, fmt.Errorf("replace SVG: %w", err)
	}
	result.OutputBytes = len(candidate)
	result.SavingsBytes = len(original) - len(candidate)
	result.SavingsPercent = float64(result.SavingsBytes) * 100 / float64(len(original))
	result.Changed = true
	return result, nil
}

func optimizeSVGDocument(ctx context.Context, input []byte) ([]byte, []string, error) {
	root, err := parseSVG(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	policy, err := validateSVG(root)
	if err != nil {
		return nil, nil, err
	}
	if !policy.styled {
		cleanSVG(root, false)
	}
	var output strings.Builder
	writeSVG(&output, root)
	candidate := []byte(output.String())
	// Independently reparse the serialized candidate and verify every local
	// reference before the shared atomic replacement path can touch the source.
	check, err := parseSVG(ctx, candidate)
	if err == nil {
		_, err = validateSVG(check)
	}
	if err != nil {
		return nil, policy.diagnostics, fmt.Errorf("invalid SVG optimizer output: %w", err)
	}
	return candidate, policy.diagnostics, nil
}
