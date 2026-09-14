package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fabricum/back-end/internal/editor"
	"fabricum/back-end/internal/processing"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const ModelReceiptSchemaVersion = 1

// ModelReceipt describes one static model optimization and every file emitted
// for the requested output.
type ModelReceipt struct {
	SchemaVersion int                                 `json:"schemaVersion"`
	Processor     string                              `json:"processor"`
	Tool          string                              `json:"tool"`
	ToolVersion   string                              `json:"toolVersion"`
	Request       processing.ModelOptimizationRequest `json:"request"`
	Outputs       []processing.ModelOutputMeasurement `json:"outputs"`
	Warnings      []string                            `json:"warnings,omitempty"`
}

// RunModelOptimization prepares, writes, and reports a static model build.
func RunModelOptimization(ctx context.Context, request processing.ModelOptimizationRequest, output io.Writer) error {
	request, err := absoluteModelRequest(request, currentDirectory())
	if err != nil {
		return fmt.Errorf("model: %w", err)
	}
	return runModelRequest(ctx, request, output)
}

// RunModelOptimizationFile reads a JSON model request from path and writes a
// versioned receipt. A path of "-" reads stdin.
func RunModelOptimizationFile(ctx context.Context, path string, output io.Writer) error {
	var reader io.Reader
	base := currentDirectory()
	var file *os.File
	if path == "-" {
		reader = os.Stdin
	} else {
		if path == "" {
			return errors.New("model request path is required")
		}
		opened, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open model request: %w", err)
		}
		file = opened
		defer file.Close()
		resolvedBase, err := filepath.Abs(filepath.Dir(path))
		if err != nil {
			return fmt.Errorf("resolve model request directory: %w", err)
		}
		base = resolvedBase
		reader = file
	}
	decoder := json.NewDecoder(io.LimitReader(reader, 1<<20))
	decoder.DisallowUnknownFields()
	var request processing.ModelOptimizationRequest
	if err := decoder.Decode(&request); err != nil {
		return fmt.Errorf("decode model request: %w", err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return err
	}
	request, err := absoluteModelRequest(request, base)
	if err != nil {
		return fmt.Errorf("model request: %w", err)
	}
	return runModelRequest(ctx, request, output)
}

func runModelRequest(ctx context.Context, request processing.ModelOptimizationRequest, output io.Writer) error {
	measurements, err := processing.OptimizeModel(ctx, request)
	if err != nil {
		return fmt.Errorf("model: %w", err)
	}
	warnings := make([]string, 0)
	for _, measurement := range measurements {
		warnings = append(warnings, measurement.Warnings...)
	}
	return json.NewEncoder(output).Encode(ModelReceipt{
		SchemaVersion: ModelReceiptSchemaVersion,
		Processor:     "fabricum/" + editor.Version,
		Tool:          "gltfpack/meshoptimizer",
		ToolVersion:   processing.GltfpackVersion,
		Request:       request,
		Outputs:       measurements,
		Warnings:      uniqueStrings(warnings),
	})
}

func absoluteModelRequest(request processing.ModelOptimizationRequest, base string) (processing.ModelOptimizationRequest, error) {
	var err error
	request.Source, err = absoluteModelPath(request.Source, base)
	if err != nil {
		return processing.ModelOptimizationRequest{}, err
	}
	request.Output, err = absoluteModelPath(request.Output, base)
	if err != nil {
		return processing.ModelOptimizationRequest{}, err
	}
	for _, path := range []*string{&request.GltfpackDirectory, &request.EncoderDirectory} {
		*path, err = absoluteModelOptionalPath(*path, base)
		if err != nil {
			return processing.ModelOptimizationRequest{}, err
		}
	}
	return request, nil
}

func absoluteModelPath(path, base string) (string, error) {
	if path == "" {
		return "", nil
	}
	return absoluteModelOptionalPath(path, base)
}

func absoluteModelOptionalPath(path, base string) (string, error) {
	if path == "" || filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Abs(filepath.Join(base, path))
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
