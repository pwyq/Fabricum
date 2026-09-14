package processing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// OptimizeModel prepares, optimizes, validates, and atomically writes a
// static model and any loose glTF sidecars emitted by gltfpack.
func OptimizeModel(ctx context.Context, request ModelOptimizationRequest) ([]ModelOutputMeasurement, error) {
	result, err := PrepareModelOutputs(ctx, request)
	if err != nil {
		return nil, err
	}
	for _, output := range result.Outputs {
		if err := WriteFileAtomically(output.Path, output.Data); err != nil {
			return nil, fmt.Errorf("write model output %s: %w", output.Measurement.Path, err)
		}
	}
	measurements := make([]ModelOutputMeasurement, len(result.Outputs))
	for index, output := range result.Outputs {
		measurements[index] = output.Measurement
	}
	return measurements, nil
}

// PrepareModelOutputs returns model bytes and measurements without replacing
// any delivery files.
func PrepareModelOutputs(ctx context.Context, request ModelOptimizationRequest) (ModelOptimizationResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	normalized, err := normalizeModelRequest(request)
	if err != nil {
		return ModelOptimizationResult{}, err
	}
	document, err := loadModelDocument(normalized.Source)
	if err != nil {
		return ModelOptimizationResult{}, err
	}
	warnings, err := prepareModelGeometry(&document, normalized)
	if err != nil {
		return ModelOptimizationResult{}, err
	}
	warnings = append(warnings, unsupportedModelWarnings(document.Object)...)
	workspace, err := os.MkdirTemp("", "fabricum-gltfpack-")
	if err != nil {
		return ModelOptimizationResult{}, fmt.Errorf("create model workspace: %w", err)
	}
	defer os.RemoveAll(workspace)
	if err := stageModelImages(&document, workspace); err != nil {
		return ModelOptimizationResult{}, err
	}
	inputPath := filepath.Join(workspace, "input.gltf")
	if err := writeModelJSON(inputPath, document.Object); err != nil {
		return ModelOptimizationResult{}, err
	}
	outputFormat := modelOutputFormat(normalized.Output)
	resultPath := filepath.Join(workspace, "result."+outputFormat)
	executable, err := findGltfpack(normalized)
	if err != nil {
		return ModelOptimizationResult{}, err
	}
	stderr, err := runGltfpack(ctx, executable, normalized, inputPath, resultPath, workspace)
	if err != nil {
		return ModelOptimizationResult{}, err
	}
	warnings = append(warnings, toolWarnings(stderr)...)
	outputs, err := readModelOutputs(resultPath, normalized.Output, normalized, warnings)
	if err != nil {
		return ModelOptimizationResult{}, err
	}
	return ModelOptimizationResult{Outputs: outputs, Warnings: warnings}, nil
}
