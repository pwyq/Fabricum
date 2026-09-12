package fabricum

import (
	"errors"
	"fmt"
	"image"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Version identifies processing behavior in export records.
const Version = "0.1.0"

// Source supplies explicit paths for an image and its two delivery roles.
type Source struct {
	Path         string `json:"path"`
	SquareOutput string `json:"squareOutput"`
	WideOutput   string `json:"wideOutput"`
}

// Config sets processing sizes and optional host-owned source and export policy.
// Paths are relative to the working directory. Callbacks run serially.
type Config struct {
	Address          string
	Source           string
	SourceSize       int
	SquareSize       int
	WideWidth        int
	OutputDirectory  string
	SquareOutput     string
	WideOutput       string
	EncoderDirectory string
	Sources          func() ([]Source, error)
	AfterExport      func(string, ExportRequest, []OutputMeasurement) error
}

type ExportRequest = exportRequest
type CropRect = cropRect
type OutputMeasurement = outputMeasurement

type processorConfig struct {
	sourcePath       string
	sourceSize       int
	encoderDirectory string
	squareOutput     outputSpec
	wideOutput       outputSpec
	outputDirectory  string
	squarePath       string
	widePath         string
	sources          func() ([]Source, error)
	afterExport      func(string, ExportRequest, []OutputMeasurement) error
}

func configure(options Config) (processorConfig, error) {
	if options.SourceSize < 0 {
		return processorConfig{}, errors.New("source-size cannot be negative")
	}
	if options.SquareSize == 0 {
		options.SquareSize = 512
	}
	if options.WideWidth == 0 {
		options.WideWidth = 768
	}
	if options.SquareSize < 1 || options.WideWidth < 4 || options.WideWidth%4 != 0 || options.SquareSize > 8192 || options.WideWidth > 8192 {
		return processorConfig{}, errors.New("output sizes must be 1..8192; wide-width must be a multiple of four")
	}
	if options.OutputDirectory == "" {
		options.OutputDirectory = "output"
	}
	directory, err := filepath.Abs(options.OutputDirectory)
	if err != nil {
		return processorConfig{}, err
	}
	config := processorConfig{
		sourceSize: options.SourceSize, encoderDirectory: options.EncoderDirectory,
		outputDirectory: directory, squarePath: options.SquareOutput, widePath: options.WideOutput,
		squareOutput: outputSpec{role: "square", width: options.SquareSize, height: options.SquareSize},
		wideOutput:   outputSpec{role: "wide", width: options.WideWidth, height: options.WideWidth * 3 / 4},
		sources:      options.Sources, afterExport: options.AfterExport,
	}
	if options.Source == "" {
		if options.Sources == nil {
			return processorConfig{}, errors.New("source is required; pass -source or configure a source list")
		}
		return config, nil
	}
	return config.withSource(options.Source)
}

func (config processorConfig) withSource(path string) (processorConfig, error) {
	if path == "" {
		return processorConfig{}, errors.New("source path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return processorConfig{}, err
	}
	source := Source{Path: abs, SquareOutput: config.squarePath, WideOutput: config.widePath}
	if config.sources != nil {
		sources, err := config.sources()
		if err != nil {
			return processorConfig{}, err
		}
		found := false
		for _, candidate := range sources {
			candidatePath, err := filepath.Abs(candidate.Path)
			if err != nil {
				return processorConfig{}, err
			}
			if candidatePath == abs {
				source = candidate
				source.Path = abs
				found = true
				break
			}
		}
		if !found {
			return processorConfig{}, errors.New("source is not in the configured source list")
		}
	}
	base := filepath.Base(abs)
	base = base[:len(base)-len(filepath.Ext(base))]
	if source.SquareOutput == "" {
		source.SquareOutput = filepath.Join(config.outputDirectory, base+"-square.webp")
	}
	if source.WideOutput == "" {
		source.WideOutput = filepath.Join(config.outputDirectory, base+"-wide.webp")
	}
	config.sourcePath = source.Path
	config.squareOutput.path, err = filepath.Abs(source.SquareOutput)
	if err != nil {
		return processorConfig{}, err
	}
	config.wideOutput.path, err = filepath.Abs(source.WideOutput)
	if err != nil {
		return processorConfig{}, err
	}
	for _, format := range []string{"png", "webp", "avif"} {
		square, wide := outputPath(config.squareOutput.path, format), outputPath(config.wideOutput.path, format)
		if samePath(square, wide) || samePath(square, abs) || samePath(wide, abs) {
			return processorConfig{}, errors.New("source and output paths must be distinct for every format")
		}
	}
	return config, nil
}

func samePath(left, right string) bool {
	if (runtime.GOOS == "windows" && strings.EqualFold(filepath.Clean(left), filepath.Clean(right))) || filepath.Clean(left) == filepath.Clean(right) {
		return true
	}
	l, le := os.Stat(left)
	r, re := os.Stat(right)
	return le == nil && re == nil && os.SameFile(l, r)
}

func validateLocalAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return errors.New("address must use localhost or a loopback IP")
	}
	return nil
}

func readSourceConfig(path string) (image.Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return image.Config{}, fmt.Errorf("open source: %w", err)
	}
	defer file.Close()
	decoded, _, err := image.DecodeConfig(file)
	if err != nil {
		return image.Config{}, fmt.Errorf("decode source configuration: %w", err)
	}
	return decoded, nil
}

func validateSourceSize(source image.Config, config processorConfig) error {
	if source.Width < 1 || source.Height < 1 || int64(source.Width)*int64(source.Height) > 64*1024*1024 {
		return errors.New("source must contain between 1 and 67108864 pixels")
	}
	if config.sourceSize > 0 && (source.Width != config.sourceSize || source.Height != config.sourceSize) {
		return fmt.Errorf("source must be exactly %dx%d, got %dx%d", config.sourceSize, config.sourceSize, source.Width, source.Height)
	}
	for _, output := range []outputSpec{config.squareOutput, config.wideOutput} {
		if source.Width < output.width || source.Height < output.height {
			return fmt.Errorf("source %dx%d is smaller than %s output %dx%d", source.Width, source.Height, output.role, output.width, output.height)
		}
	}
	return nil
}
