package editor

import (
	"errors"
	"fabricum/back-end/internal/processing"
	"fmt"
	"image"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Version identifies processing behavior in export records.
const Version = "0.2.2"

const (
	// ModeGUI opens the local editor and allows a source to be chosen there.
	ModeGUI = "gui"
	// ModeCLI keeps the configured, path-driven server behavior without opening a browser.
	ModeCLI = "cli"
)

// Source supplies explicit paths for an image and its two delivery roles.
type Source struct {
	Path         string `json:"path"`
	SquareOutput string `json:"squareOutput"`
	WideOutput   string `json:"wideOutput"`
}

// Config sets the launch mode, processing sizes, and optional host-owned source
// and export policy. An empty Mode defaults to GUI. Paths are relative to the
// working directory. Callbacks run serially.
type Config struct {
	Mode             string
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
	AfterExport      func(string, processing.ExportRequest, []processing.OutputMeasurement) error
}

type processorConfig struct {
	mode             string
	sourcePath       string
	sourceSize       int
	encoderDirectory string
	squareOutput     processing.OutputSpec
	wideOutput       processing.OutputSpec
	outputDirectory  string
	squarePath       string
	widePath         string
	sources          func() ([]Source, error)
	afterExport      func(string, processing.ExportRequest, []processing.OutputMeasurement) error
}

func configure(options Config) (processorConfig, error) {
	mode := options.Mode
	if mode == "" {
		mode = ModeGUI
	}
	if mode != ModeGUI && mode != ModeCLI {
		return processorConfig{}, errors.New("mode must be gui or cli")
	}
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
		mode:       mode,
		sourceSize: options.SourceSize, encoderDirectory: options.EncoderDirectory,
		outputDirectory: directory, squarePath: options.SquareOutput, widePath: options.WideOutput,
		squareOutput: processing.OutputSpec{Role: "square", Width: options.SquareSize, Height: options.SquareSize},
		wideOutput:   processing.OutputSpec{Role: "wide", Width: options.WideWidth, Height: options.WideWidth * 3 / 4},
		sources:      options.Sources, afterExport: options.AfterExport,
	}
	if options.Source == "" {
		if mode == ModeCLI {
			return processorConfig{}, errors.New("source is required in cli mode; pass --source or -s")
		}
		return config, nil
	}
	return config.withSource(options.Source)
}

// ValidateConfig checks launch and processing configuration without starting the editor.
func ValidateConfig(options Config) error {
	if options.Address != "" {
		if err := validateLocalAddress(options.Address); err != nil {
			return err
		}
	}
	_, err := configure(options)
	return err
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
	config.squareOutput.Path, err = filepath.Abs(source.SquareOutput)
	if err != nil {
		return processorConfig{}, err
	}
	config.wideOutput.Path, err = filepath.Abs(source.WideOutput)
	if err != nil {
		return processorConfig{}, err
	}
	for _, format := range []string{"png", "webp", "avif"} {
		square, wide := processing.OutputPath(config.squareOutput.Path, format), processing.OutputPath(config.wideOutput.Path, format)
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
	for _, output := range []processing.OutputSpec{config.squareOutput, config.wideOutput} {
		if source.Width < output.Width || source.Height < output.Height {
			return fmt.Errorf("source %dx%d is smaller than %s output %dx%d", source.Width, source.Height, output.Role, output.Width, output.Height)
		}
	}
	return nil
}
