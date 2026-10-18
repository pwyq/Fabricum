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
const SVGOptimizerVersion = "1.1.0"

// SVGOptimizationOptions keeps lossy numeric rounding and UI color changes opt-in.
type SVGOptimizationOptions struct {
	Icon              bool `json:"icon,omitempty"`
	Precision         *int `json:"precision,omitempty"`
	SelectiveRounding bool `json:"selectiveRounding,omitempty"`
	CurrentColor      bool `json:"currentColor,omitempty"`
}

type SVGOptimizationRequest struct {
	Path       string `json:"path"`
	OutputPath string `json:"outputPath,omitempty"`
	SVGOptimizationOptions
}

type SVGOptimizationResult struct {
	Path             string         `json:"path"`
	OutputPath       string         `json:"outputPath,omitempty"`
	InputBytes       int            `json:"inputBytes"`
	OutputBytes      int            `json:"outputBytes"`
	SavingsBytes     int            `json:"savingsBytes"`
	SavingsPercent   float64        `json:"savingsPercent"`
	Format           string         `json:"format"`
	Optimizer        string         `json:"optimizer"`
	OptimizerVersion string         `json:"optimizerVersion"`
	Changed          bool           `json:"changed"`
	Diagnostics      []string       `json:"diagnostics,omitempty"`
	Error            string         `json:"error,omitempty"`
	PathErrors       []SVGPathError `json:"pathErrors,omitempty"`
}

// OptimizeSVG optimizes a static SVG offline. OutputPath creates a new file;
// otherwise only a strictly smaller candidate replaces the original.
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
	if request.Precision != nil && (*request.Precision < 0 || *request.Precision > 8) {
		return result, fmt.Errorf("SVG precision must be between 0 and 8")
	}
	path, err := filepath.Abs(request.Path)
	if err != nil {
		return result, err
	}
	result.Path = path
	if request.OutputPath != "" {
		result.OutputPath, err = filepath.Abs(request.OutputPath)
		if err != nil {
			return result, err
		}
		if err := svgNewOutputPath(path, result.OutputPath); err != nil {
			return result, err
		}
	}
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
	candidate, diagnostics, pathErrors, err := optimizeSVGWithOptions(ctx, original, request.SVGOptimizationOptions)
	result.Diagnostics = diagnostics
	if err != nil {
		return result, err
	}
	result.Format = "svg"
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if len(candidate) >= len(original) {
		candidate = original
	}
	if result.OutputPath != "" {
		err = writeNewSVG(ctx, result.OutputPath, candidate, info)
	} else if len(candidate) < len(original) {
		err = replaceOptimizedFile(ctx, path, original, candidate, info)
	} else {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("write SVG: %w", err)
	}
	result.OutputBytes = len(candidate)
	result.SavingsBytes = len(original) - len(candidate)
	result.SavingsPercent = float64(result.SavingsBytes) * 100 / float64(len(original))
	result.Changed = result.SavingsBytes > 0
	if result.Changed {
		result.PathErrors = pathErrors
	}
	return result, nil
}

func optimizeSVGDocument(ctx context.Context, input []byte) ([]byte, []string, error) {
	output, diagnostics, _, err := optimizeSVGWithOptions(ctx, input, SVGOptimizationOptions{})
	return output, diagnostics, err
}

func optimizeSVGWithOptions(ctx context.Context, input []byte, options SVGOptimizationOptions) ([]byte, []string, []SVGPathError, error) {
	root, err := parseSVG(ctx, input)
	if err != nil {
		return nil, nil, nil, err
	}
	policy, err := validateSVG(root)
	if err != nil {
		return nil, nil, nil, err
	}
	iconPrepared := options.Icon && svgIconStylesSupported(root)
	if iconPrepared {
		prepareSVGIcon(root, policy)
		policy, err = validateSVG(root)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	if policy.styled && (options.Precision != nil || options.SelectiveRounding || options.CurrentColor) {
		return nil, policy.diagnostics, nil, fmt.Errorf("rounding/color options require SVGs without classes or stylesheets; --icon supports simple inline presentation styles")
	}
	if options.CurrentColor {
		if !svgCurrentColor(root) {
			policy.diagnostics = append(policy.diagnostics, "Paints retained: currentColor requires one recognized opaque solid color on simple geometry, without definitions, text, or indirect paints.")
		}
	}
	if err := roundSVGPresentation(root, options); err != nil {
		return nil, policy.diagnostics, nil, err
	}
	if iconPrepared {
		cleanSVGIconPresentation(root, policy)
	}
	if !policy.styled {
		cleanSVG(root, false)
	}
	var pathErrors []SVGPathError
	if !policy.styled && (options.Icon || options.Precision != nil || options.SelectiveRounding) {
		pathErrors, err = optimizeSVGPaths(root, options)
		if err != nil {
			return nil, policy.diagnostics, nil, err
		}
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
		return nil, policy.diagnostics, nil, fmt.Errorf("invalid SVG optimizer output: %w", err)
	}
	return candidate, policy.diagnostics, pathErrors, nil
}
