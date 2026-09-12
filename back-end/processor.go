package fabricum

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
)

type cropRect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type exportRequest struct {
	Square   cropRect `json:"square"`
	Wide     cropRect `json:"wide"`
	Format   string   `json:"format"`
	Quality  int      `json:"quality"`
	Lossless bool     `json:"lossless"`
}

type outputSpec struct {
	role   string
	path   string
	width  int
	height int
}

type outputMeasurement struct {
	Role     string   `json:"role"`
	Path     string   `json:"path"`
	Width    int      `json:"width"`
	Height   int      `json:"height"`
	Format   string   `json:"format"`
	Quality  int      `json:"quality,omitempty"`
	Lossless bool     `json:"lossless,omitempty"`
	Bytes    int      `json:"bytes"`
	SHA256   string   `json:"sha256"`
	Crop     cropRect `json:"crop"`
}

type processedOutput struct {
	measurement outputMeasurement
	path        string
	data        []byte
}

func processOutputs(sourcePath string, request exportRequest, specs []outputSpec, encoderDirectory string) ([]outputMeasurement, error) {
	outputs, err := prepareOutputs(context.Background(), sourcePath, request, specs, encoderDirectory)
	if err != nil {
		return nil, err
	}
	if err := writeOutputs(outputs); err != nil {
		return nil, err
	}
	return outputMeasurements(outputs), nil
}

func prepareOutputs(ctx context.Context, sourcePath string, request exportRequest, specs []outputSpec, encoderDirectory string) ([]processedOutput, error) {
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
	crops := map[string]cropRect{"square": request.Square, "wide": request.Wide}
	outputs := make([]processedOutput, 0, len(specs))
	for _, spec := range specs {
		crop := crops[spec.role]
		if err := validateCrop(source.Bounds(), crop, spec); err != nil {
			return nil, fmt.Errorf("%s crop: %w", spec.role, err)
		}
		resized := resizeCrop(source, crop, spec.width, spec.height)
		data, err := encodeOutput(ctx, resized, request, encoderDirectory)
		if err != nil {
			return nil, fmt.Errorf("encode %s output: %w", spec.role, err)
		}
		hash := sha256.Sum256(data)
		path := outputPath(spec.path, request.Format)
		outputs = append(outputs, processedOutput{
			path: path, data: data,
			measurement: outputMeasurement{
				Role: spec.role, Path: filepath.ToSlash(path), Width: spec.width, Height: spec.height,
				Format: request.Format, Quality: request.Quality, Lossless: request.Lossless,
				Bytes: len(data), SHA256: hex.EncodeToString(hash[:]), Crop: crop,
			},
		})
	}
	return outputs, nil
}

func writeOutputs(outputs []processedOutput) error {
	for _, output := range outputs {
		if err := writeFileAtomically(output.path, output.data); err != nil {
			return fmt.Errorf("write %s output: %w", output.measurement.Role, err)
		}
	}
	return nil
}

func outputMeasurements(outputs []processedOutput) []outputMeasurement {
	measurements := make([]outputMeasurement, len(outputs))
	for index, output := range outputs {
		measurements[index] = output.measurement
	}
	return measurements
}

func validateCrop(bounds image.Rectangle, crop cropRect, spec outputSpec) error {
	if crop.Width <= 0 || crop.Height <= 0 {
		return errors.New("dimensions must be positive")
	}
	if crop.X < 0 || crop.Y < 0 || crop.Width > bounds.Dx() || crop.Height > bounds.Dy() || crop.X > bounds.Dx()-crop.Width || crop.Y > bounds.Dy()-crop.Height {
		return fmt.Errorf("rectangle %+v exceeds %dx%d source", crop, bounds.Dx(), bounds.Dy())
	}
	if crop.Width*spec.height != crop.Height*spec.width {
		return fmt.Errorf("rectangle must use %d:%d aspect ratio", spec.width, spec.height)
	}
	if crop.Width < spec.width || crop.Height < spec.height {
		return fmt.Errorf("rectangle must be at least %dx%d to avoid upscaling", spec.width, spec.height)
	}
	return nil
}

func resizeCrop(source image.Image, crop cropRect, width, height int) *image.NRGBA {
	result := image.NewNRGBA(image.Rect(0, 0, width, height))
	for destinationY := 0; destinationY < height; destinationY++ {
		sourceY := float64(crop.Y) + (float64(destinationY)+0.5)*float64(crop.Height)/float64(height) - 0.5
		for destinationX := 0; destinationX < width; destinationX++ {
			sourceX := float64(crop.X) + (float64(destinationX)+0.5)*float64(crop.Width)/float64(width) - 0.5
			result.SetNRGBA(destinationX, destinationY, bilinearSample(source, sourceX, sourceY, crop))
		}
	}
	return result
}

func bilinearSample(source image.Image, sourceX, sourceY float64, crop cropRect) color.NRGBA {
	minX, minY := crop.X, crop.Y
	maxX, maxY := crop.X+crop.Width-1, crop.Y+crop.Height-1
	x0 := clamp(int(math.Floor(sourceX)), minX, maxX)
	y0 := clamp(int(math.Floor(sourceY)), minY, maxY)
	x1, y1 := clamp(x0+1, minX, maxX), clamp(y0+1, minY, maxY)
	xWeight, yWeight := sourceX-float64(x0), sourceY-float64(y0)
	if x0 == x1 {
		xWeight = 0
	}
	if y0 == y1 {
		yWeight = 0
	}
	top := interpolateColor(nrgbaAt(source, x0, y0), nrgbaAt(source, x1, y0), xWeight)
	bottom := interpolateColor(nrgbaAt(source, x0, y1), nrgbaAt(source, x1, y1), xWeight)
	return interpolateColor(top, bottom, yWeight)
}

func nrgbaAt(source image.Image, x, y int) color.NRGBA {
	return color.NRGBAModel.Convert(source.At(x+source.Bounds().Min.X, y+source.Bounds().Min.Y)).(color.NRGBA)
}

func interpolateColor(left, right color.NRGBA, weight float64) color.NRGBA {
	return color.NRGBA{
		R: interpolateChannel(left.R, right.R, weight), G: interpolateChannel(left.G, right.G, weight),
		B: interpolateChannel(left.B, right.B, weight), A: interpolateChannel(left.A, right.A, weight),
	}
}

func interpolateChannel(left, right uint8, weight float64) uint8 {
	return uint8(math.Round(float64(left)*(1-weight) + float64(right)*weight))
}

func clamp(value, minimum, maximum int) int {
	return min(max(value, minimum), maximum)
}

func writeFileAtomically(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".unit-art-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.Write(data); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return os.Rename(temporaryPath, path)
	} else if err != nil {
		return err
	}
	backup, err := os.CreateTemp(directory, ".unit-art-backup-*")
	if err != nil {
		return err
	}
	backupPath := backup.Name()
	defer os.Remove(backupPath)
	if err := backup.Close(); err != nil {
		return err
	}
	if err := os.Remove(backupPath); err != nil {
		return err
	}
	if err := os.Rename(path, backupPath); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		_ = os.Rename(backupPath, path)
		return err
	}
	return os.Remove(backupPath)
}
