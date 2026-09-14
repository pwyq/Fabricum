package processing

import (
	"context"
	"fmt"
	"strings"
)

// Transform prepares and atomically writes every output in request. No output
// is written until all source validation, transformations, and encoding have
// succeeded.
func Transform(ctx context.Context, request TransformRequest) ([]OutputMeasurement, error) {
	outputs, err := PrepareTransformOutputs(ctx, request)
	if err != nil {
		return nil, err
	}
	if err := WriteOutputs(outputs); err != nil {
		return nil, err
	}
	return OutputMeasurements(outputs), nil
}

// PrepareTransformOutputs decodes and encodes a transform request without
// writing its outputs. It is useful for previews and for callers that need to
// inspect output bytes before committing them.
func PrepareTransformOutputs(ctx context.Context, request TransformRequest) ([]ProcessedOutput, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	request.Format = strings.ToLower(strings.TrimSpace(request.Format))
	if request.Format == "" {
		request.Format = "png"
	}
	if err := validateTransformRequest(request); err != nil {
		return nil, err
	}
	if err := validateTransformPaths(request); err != nil {
		return nil, err
	}
	primary, _, err := readTransformImage(request.Source, request.effectiveConstraints())
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	outputs := make([]ProcessedOutput, 0, len(request.Outputs))
	for index, spec := range request.Outputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		imageOutput, filter, err := renderTransform(primary, spec.Transform)
		if err != nil {
			return nil, fmt.Errorf("output %s: %w", outputRole(spec, index), err)
		}
		data, err := encodeOutput(ctx, imageOutput, ExportRequest{
			Format: request.Format, PNGMode: request.PNGMode, Quality: request.Quality, Lossless: request.Lossless,
		}, request.EncoderDirectory)
		if err != nil {
			return nil, fmt.Errorf("encode %s output: %w", outputRole(spec, index), err)
		}
		path := OutputPath(spec.Path, request.Format)
		crop := CropRect{}
		if spec.Transform.Crop != nil {
			crop = *spec.Transform.Crop
		}
		outputs = append(outputs, ProcessedOutput{
			Path:        path,
			Data:        data,
			Measurement: newOutputMeasurement(outputRole(spec, index), path, imageOutput, request.Format, request.PNGMode, request.Quality, request.Lossless, filter, crop, data),
		})
	}
	return outputs, nil
}
