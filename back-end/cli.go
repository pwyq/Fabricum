package fabricum

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type fileConfig struct {
	Sources       []Source `json:"sources"`
	ExportCommand []string `json:"exportCommand"`

	Source           string `json:"source"`
	SourceSize       int    `json:"sourceSize"`
	SquareSize       int    `json:"squareSize"`
	WideWidth        int    `json:"wideWidth"`
	OutputDirectory  string `json:"outputDirectory"`
	SquareOutput     string `json:"squareOutput"`
	WideOutput       string `json:"wideOutput"`
	EncoderDirectory string `json:"encoderDirectory"`
	Address          string `json:"address"`
}

// ParseConfig loads optional JSON settings; explicit flags take precedence.
// File paths in JSON are relative to that file; CLI paths are relative to cwd.
func ParseConfig(args []string, output io.Writer) (Config, error) {
	var settings fileConfig
	commandDirectory, err := os.Getwd()
	if err != nil {
		return Config{}, err
	}
	flags := flag.NewFlagSet("fabricum", flag.ContinueOnError)
	flags.SetOutput(output)
	configFile := flags.String("config", "", "JSON configuration file")
	flags.StringVar(&settings.Source, "source", "", "input PNG, JPEG, or GIF path")
	flags.StringVar(&settings.OutputDirectory, "output-dir", "output", "output directory")
	flags.StringVar(&settings.SquareOutput, "square-output", "", "explicit square output path; extension follows selected format")
	flags.StringVar(&settings.WideOutput, "wide-output", "", "explicit wide output path; extension follows selected format")
	flags.IntVar(&settings.SourceSize, "source-size", 0, "required square source size; 0 accepts arbitrary dimensions")
	flags.IntVar(&settings.SquareSize, "square-size", 512, "square output width and height")
	flags.IntVar(&settings.WideWidth, "wide-width", 768, "4:3 output width, a multiple of four")
	flags.StringVar(&settings.EncoderDirectory, "encoder-directory", "", "directory containing installed Sharp dependencies; defaults to cwd")
	flags.StringVar(&settings.Address, "address", "127.0.0.1:4179", "loopback listen address")
	version := flags.Bool("version", false, "print version and exit")
	if err := flags.Parse(args); err != nil {
		return Config{}, err
	}
	if *version {
		fmt.Fprintln(output, "fabricum/"+Version)
		return Config{}, flag.ErrHelp
	}
	if flags.NArg() != 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	if *configFile != "" {
		file, err := os.Open(*configFile)
		if err != nil {
			return Config{}, err
		}
		defer file.Close()
		decoder := json.NewDecoder(io.LimitReader(file, 64<<10))
		decoder.DisallowUnknownFields()
		var loaded fileConfig
		if err := decoder.Decode(&loaded); err != nil {
			return Config{}, fmt.Errorf("decode config: %w", err)
		}
		if err := ensureJSONEnd(decoder); err != nil {
			return Config{}, err
		}
		root, err := filepath.Abs(filepath.Dir(*configFile))
		if err != nil {
			return Config{}, err
		}
		for _, value := range []*string{&loaded.Source, &loaded.OutputDirectory, &loaded.SquareOutput, &loaded.WideOutput, &loaded.EncoderDirectory} {
			if *value != "" && !filepath.IsAbs(*value) {
				*value = filepath.Join(root, *value)
			}
		}
		// Defaults are loaded first, then explicit command-line options override them.
		if loaded.OutputDirectory == "" {
			loaded.OutputDirectory = filepath.Join(root, "output")
		}
		if loaded.SquareSize == 0 {
			loaded.SquareSize = 512
		}
		if loaded.WideWidth == 0 {
			loaded.WideWidth = 768
		}
		if loaded.Address == "" {
			loaded.Address = "127.0.0.1:4179"
		}
		commandDirectory = root
		for i := range loaded.Sources {
			for _, value := range []*string{&loaded.Sources[i].Path, &loaded.Sources[i].SquareOutput, &loaded.Sources[i].WideOutput} {
				if *value != "" && !filepath.IsAbs(*value) {
					*value = filepath.Join(root, *value)
				}
			}
		}
		settings = loaded
		if err := flags.Parse(args); err != nil {
			return Config{}, err
		}
	}
	if err := validateLocalAddress(settings.Address); err != nil {
		return Config{}, err
	}
	options := Config{Address: settings.Address, Source: settings.Source, SourceSize: settings.SourceSize,
		SquareSize: settings.SquareSize, WideWidth: settings.WideWidth, OutputDirectory: settings.OutputDirectory,
		SquareOutput: settings.SquareOutput, WideOutput: settings.WideOutput, EncoderDirectory: settings.EncoderDirectory}
	if settings.Sources != nil {
		options.Sources = func() ([]Source, error) { return settings.Sources, nil }
	}
	if len(settings.ExportCommand) > 0 {
		options.AfterExport = exportCommand(settings.ExportCommand, commandDirectory)
	}
	if _, err := configure(options); err != nil {
		return Config{}, err
	}
	return options, nil
}
