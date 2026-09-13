package processing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"os"
	"path/filepath"
)

func processOutputs(sourcePath string, request ExportRequest, specs []OutputSpec, encoderDirectory string) ([]OutputMeasurement, error) {
	outputs, err := PrepareOutputs(context.Background(), sourcePath, request, specs, encoderDirectory)
	if err != nil {
		return nil, err
	}
	if err := WriteOutputs(outputs); err != nil {
		return nil, err
	}
	return OutputMeasurements(outputs), nil
}

func PrepareOutputs(ctx context.Context, sourcePath string, request ExportRequest, specs []OutputSpec, encoderDirectory string) ([]ProcessedOutput, error) {
	if err := validateEncodingOptions(request); err != nil {
		return nil, err
	}
	file, err := os.Open(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("open source: %w", err)
	}
	defer file.Close()
	source, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode source: %w", err)
	}
	crops := map[string]CropRect{"square": request.Square, "wide": request.Wide}
	outputs := make([]ProcessedOutput, 0, len(specs))
	for _, spec := range specs {
		crop := crops[spec.Role]
		if err := validateCrop(source.Bounds(), crop, spec); err != nil {
			return nil, fmt.Errorf("%s crop: %w", spec.Role, err)
		}
		resized := resizeCrop(source, crop, spec.Width, spec.Height)
		data, err := encodeOutput(ctx, resized, request, encoderDirectory)
		if err != nil {
			return nil, fmt.Errorf("encode %s output: %w", spec.Role, err)
		}
		hash := sha256.Sum256(data)
		path := OutputPath(spec.Path, request.Format)
		outputs = append(outputs, ProcessedOutput{
			Path: path, Data: data,
			Measurement: OutputMeasurement{
				Role: spec.Role, Path: filepath.ToSlash(path), Width: spec.Width, Height: spec.Height,
				Format: request.Format, Encoder: EncoderForFormat(request.Format), Quality: request.Quality, Lossless: request.Lossless,
				HasAlpha: !imageOpaque(resized), Bytes: len(data), SHA256: hex.EncodeToString(hash[:]), Crop: crop,
			},
		})
	}
	return outputs, nil
}

func OutputMeasurements(outputs []ProcessedOutput) []OutputMeasurement {
	measurements := make([]OutputMeasurement, len(outputs))
	for index, output := range outputs {
		measurements[index] = output.Measurement
	}
	return measurements
}
