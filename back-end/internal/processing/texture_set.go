package processing

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

const (
	defaultTextureWorkers = 4
	maxTextureWorkers     = 32
)

// BuildTextureSet prepares every loose KTX2 output before replacing any of
// them. Use RequiredRoles when a caller needs a complete material set.
func BuildTextureSet(ctx context.Context, request TextureSetRequest) ([]OutputMeasurement, error) {
	outputs, err := PrepareTextureSetOutputs(ctx, request)
	if err != nil {
		return nil, err
	}
	if err := WriteOutputs(outputs); err != nil {
		return nil, err
	}
	return OutputMeasurements(outputs), nil
}

// PrepareTextureSetOutputs renders and encodes a texture set without writing
// files. Results retain request order even when encodes run concurrently.
func PrepareTextureSetOutputs(ctx context.Context, request TextureSetRequest) ([]ProcessedOutput, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateTextureSetRequest(request); err != nil {
		return nil, err
	}
	workers, err := textureWorkerCount(request.MaxWorkers, len(request.Outputs))
	if err != nil {
		return nil, err
	}
	workContext, cancel := context.WithCancel(ctx)
	defer cancel()
	outputs := make([]ProcessedOutput, len(request.Outputs))
	slots := make(chan struct{}, workers)
	var wait sync.WaitGroup
	var firstErr error
	var errorOnce sync.Once
	for index, spec := range request.Outputs {
		wait.Add(1)
		go func(index int, spec TextureOutputSpec) {
			defer wait.Done()
			select {
			case slots <- struct{}{}:
			case <-workContext.Done():
				return
			}
			defer func() { <-slots }()
			output, err := prepareTextureOutput(workContext, spec, index, request.EncoderDirectory)
			if err != nil {
				errorOnce.Do(func() {
					firstErr = fmt.Errorf("encode %s output: %w", textureOutputRole(spec, index), err)
					cancel()
				})
				return
			}
			outputs[index] = output
		}(index, spec)
	}
	wait.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return outputs, nil
}

// EncodeTexture prepares one standalone loose KTX2 output without writing it.
func EncodeTexture(ctx context.Context, spec TextureOutputSpec, encoderDirectory string) (ProcessedOutput, error) {
	outputs, err := PrepareTextureSetOutputs(ctx, TextureSetRequest{
		Outputs:          []TextureOutputSpec{spec},
		MaxWorkers:       1,
		EncoderDirectory: encoderDirectory,
	})
	if err != nil {
		return ProcessedOutput{}, err
	}
	return outputs[0], nil
}

func textureWorkerCount(requested, total int) (int, error) {
	if requested < 0 {
		return 0, errors.New("maxWorkers cannot be negative")
	}
	if requested == 0 {
		requested = defaultTextureWorkers
	}
	if requested > maxTextureWorkers {
		return 0, fmt.Errorf("maxWorkers cannot exceed %d", maxTextureWorkers)
	}
	if requested > total {
		requested = total
	}
	return requested, nil
}

func validateTextureSetRequest(request TextureSetRequest) error {
	if len(request.Outputs) == 0 {
		return errors.New("at least one texture output is required")
	}
	if len(request.Outputs) > maxTransformOutputs {
		return fmt.Errorf("at most %d texture outputs are allowed", maxTransformOutputs)
	}
	if _, err := textureWorkerCount(request.MaxWorkers, len(request.Outputs)); err != nil {
		return err
	}
	roles := make(map[string]struct{}, len(request.Outputs))
	for index, spec := range request.Outputs {
		if spec.Source == "" || len(spec.Source) > maxTransformPath {
			return fmt.Errorf("texture output %d source path must contain between 1 and 4096 bytes", index+1)
		}
		if spec.Path == "" || len(spec.Path) > maxTransformPath {
			return fmt.Errorf("texture output %d path must contain between 1 and 4096 bytes", index+1)
		}
		constraints := textureConstraints(spec)
		if constraints.Width < 0 || constraints.Height < 0 || (constraints.Width == 0) != (constraints.Height == 0) {
			return fmt.Errorf("texture output %s source width and height must both be positive or both be zero", textureOutputRole(spec, index))
		}
		if constraints.Format != "" {
			if _, ok := normalizeImageFormat(constraints.Format); !ok {
				return fmt.Errorf("texture output %s has unsupported source format %q", textureOutputRole(spec, index), constraints.Format)
			}
		}
		if spec.Format != "" && strings.ToLower(strings.TrimSpace(spec.Format)) != "ktx2" {
			return fmt.Errorf("texture output %s format must be ktx2", textureOutputRole(spec, index))
		}
		if err := validateImageTransform(spec.Transform); err != nil {
			return fmt.Errorf("texture output %s: %w", textureOutputRole(spec, index), err)
		}
		role := textureOutputRole(spec, index)
		if _, exists := roles[role]; exists {
			return fmt.Errorf("texture output roles must be distinct: %s", role)
		}
		roles[role] = struct{}{}
	}
	for _, role := range request.RequiredRoles {
		if role == "" {
			return errors.New("required texture roles cannot be empty")
		}
		if _, ok := roles[role]; !ok {
			return fmt.Errorf("required texture role %q is missing", role)
		}
	}
	return validateTexturePaths(request)
}

func validateTexturePaths(request TextureSetRequest) error {
	outputs := make([]string, len(request.Outputs))
	for index, spec := range request.Outputs {
		path, err := filepath.Abs(OutputPath(spec.Path, "ktx2"))
		if err != nil {
			return fmt.Errorf("resolve %s output path: %w", textureOutputRole(spec, index), err)
		}
		for previous := 0; previous < index; previous++ {
			if sameTransformPath(outputs[previous], path) {
				return errors.New("texture output paths must be distinct")
			}
		}
		outputs[index] = path
		for _, input := range textureInputPaths(spec) {
			absolute, err := filepath.Abs(input)
			if err != nil {
				return fmt.Errorf("resolve texture input path: %w", err)
			}
			if sameTransformPath(absolute, path) {
				return errors.New("texture input and output paths must be distinct")
			}
		}
	}
	return nil
}

func textureInputPaths(spec TextureOutputSpec) []string {
	paths := []string{spec.Source}
	if pack := spec.Transform.Pack; pack != nil {
		for _, input := range []ChannelInput{pack.Red, pack.Green, pack.Blue} {
			if input.Source != "" {
				paths = append(paths, input.Source)
			}
		}
	}
	return paths
}

func textureOutputRole(spec TextureOutputSpec, index int) string {
	if spec.Role != "" {
		return spec.Role
	}
	return fmt.Sprintf("output-%d", index+1)
}

func prepareTextureOutput(ctx context.Context, spec TextureOutputSpec, index int, encoderDirectory string) (ProcessedOutput, error) {
	primary, _, err := readTransformImage(spec.Source, textureConstraints(spec))
	if err != nil {
		return ProcessedOutput{}, fmt.Errorf("source: %w", err)
	}
	imageOutput, filter, err := renderTransform(primary, spec.Transform)
	if err != nil {
		return ProcessedOutput{}, err
	}
	if !imageOpaque(imageOutput) {
		return ProcessedOutput{}, errors.New("texture output must be opaque RGB; remove alpha or pack RGB channels")
	}
	data, encoding, err := encodeKTX2Image(ctx, imageOutput, spec.Encoding, encoderDirectory)
	if err != nil {
		return ProcessedOutput{}, err
	}
	path := OutputPath(spec.Path, "ktx2")
	crop := CropRect{}
	if spec.Transform.Crop != nil {
		crop = *spec.Transform.Crop
	}
	measurement := newOutputMeasurement(textureOutputRole(spec, index), path, imageOutput, "ktx2", encoding.Quality, false, filter, crop, data)
	measurement.Encoding = encoding.Encoding
	measurement.MipLevels = encoding.MipLevels
	measurement.TransferFunction = encoding.TransferFunction
	measurement.ColorPrimaries = encoding.ColorPrimaries
	measurement.Processor = "fabricum"
	measurement.EncoderVersion = BasisUniversalVersion
	measurement.NativeEncoderVersions = EncoderVersionsForFormat("ktx2")
	return ProcessedOutput{Measurement: measurement, Path: path, Data: data}, nil
}

func textureConstraints(spec TextureOutputSpec) SourceConstraints {
	if spec.SourceConstraints != nil {
		return *spec.SourceConstraints
	}
	return spec.Constraints
}
