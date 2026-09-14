package compatibility

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strings"

	fabricum "fabricum/back-end"
)

func runModelWorkflow(ctx context.Context, fixture fixtures) ([]fabricum.ModelReceipt, error) {
	receipts := make([]fabricum.ModelReceipt, 0, 2)
	for _, variant := range []struct {
		name, output, compression string
	}{
		{name: "transformed", output: filepath.Join(fixture.Root, "model", "transformed.gltf"), compression: fabricum.ModelCompressionNone},
		{name: "meshopt", output: filepath.Join(fixture.Root, "model", "optimized.glb"), compression: fabricum.ModelCompressionMeshopt},
	} {
		request := fabricum.ModelOptimizationRequest{
			Source: fixture.Model, Output: variant.output, Role: "synthetic-prop", Compression: variant.compression,
			BakeRootTransform: true, CenterXZAtGround: true, RemoveAttributes: []string{"COLOR_0"},
			DeduplicateMaterials: true, CompactBuffers: true,
			MaterialNames: map[string]string{"synthetic-material": "compatibility-material"},
		}
		requestPath := filepath.Join(fixture.Root, "model", variant.name+".json")
		if err := writeJSON(requestPath, request); err != nil {
			return nil, err
		}
		var receipt fabricum.ModelReceipt
		if err := runReceiptFile(ctx, requestPath, fabricum.ModelOptimizationFile, &receipt); err != nil {
			return nil, fmt.Errorf("%s model: %w", variant.name, err)
		}
		if err := validateModelReceipt(receipt, variant.compression, variant.output); err != nil {
			return nil, err
		}
		receipts = append(receipts, receipt)
	}
	return receipts, nil
}

func validateModelReceipt(receipt fabricum.ModelReceipt, compression, output string) error {
	if receipt.SchemaVersion != fabricum.ModelReceiptSchemaVersion || receipt.Processor == "" || receipt.Tool == "" || receipt.ToolVersion == "" || len(receipt.Outputs) == 0 {
		return fmt.Errorf("model returned an incomplete receipt: %+v", receipt)
	}
	mainFound := false
	meshoptFound := false
	for _, measurement := range receipt.Outputs {
		if err := checkModelMeasurement(measurement); err != nil {
			return err
		}
		if measurement.Path == filepath.ToSlash(output) {
			mainFound = true
			if measurement.Bounds == nil || measurement.TriangleCount < 1 || measurement.MaterialCount != 1 || math.Abs(measurement.Bounds.Min[1]) > 0.0001 || math.Abs(measurement.Bounds.Min[0]+measurement.Bounds.Max[0]) > 0.0001 || math.Abs(measurement.Bounds.Min[2]+measurement.Bounds.Max[2]) > 0.0001 {
				return fmt.Errorf("model bounds or counts do not match the centered prop contract: bounds=%+v triangles=%d materials=%d", measurement.Bounds, measurement.TriangleCount, measurement.MaterialCount)
			}
			for _, extension := range measurement.UsedExtensions {
				if extension == "EXT_meshopt_compression" {
					meshoptFound = true
				}
				if strings.Contains(strings.ToLower(extension), "draco") {
					return fmt.Errorf("model emitted forbidden Draco extension %s", extension)
				}
			}
		}
	}
	if !mainFound {
		return fmt.Errorf("model receipt omitted main output %s", output)
	}
	if compression == fabricum.ModelCompressionMeshopt && !meshoptFound {
		return fmt.Errorf("meshopt model did not report EXT_meshopt_compression")
	}
	return nil
}
