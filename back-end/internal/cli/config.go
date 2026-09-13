package cli

import (
	"encoding/json"
	"fabricum/back-end/internal/editor"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type fileConfig struct {
	Sources       []editor.Source `json:"sources"`
	ExportCommand []string        `json:"exportCommand"`

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
func ParseConfig(args []string, output io.Writer) (editor.Config, error) {
	var settings fileConfig
	commandDirectory, err := os.Getwd()
	if err != nil {
		return editor.Config{}, err
	}
	flags := flag.NewFlagSet("fabricum", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.Usage = func() {
		fmt.Fprintln(output, "Usage:")
		fmt.Fprintln(output, "  fabricum")
		fmt.Fprintln(output, "  fabricum --source path --output path")
		fmt.Fprintln(output, "  fabricum -s path -o path")
		fmt.Fprintln(output, "  fabricum inspect path [path ...]")
		fmt.Fprintln(output, "  fabricum transform request.json")
		fmt.Fprintln(output, "  fabricum texture-set request.json")
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Options:")
		fmt.Fprintln(output, "  -h, --help")
		fmt.Fprintln(output, "    \tshow this help")
		flags.PrintDefaults()
	}
	configFile := flags.String("config", "", "JSON configuration file")
	flags.StringVar(&settings.Source, "source", "", "input PNG, JPEG, or GIF path")
	flags.StringVar(&settings.Source, "s", "", "shorthand for --source")
	flags.StringVar(&settings.OutputDirectory, "output", "output", "output directory")
	flags.StringVar(&settings.OutputDirectory, "o", "output", "shorthand for --output")
	flags.StringVar(&settings.SquareOutput, "square-output", "", "explicit square output path; extension follows selected format")
	flags.StringVar(&settings.WideOutput, "wide-output", "", "explicit wide output path; extension follows selected format")
	flags.IntVar(&settings.SourceSize, "source-size", 0, "required square source size; 0 accepts arbitrary dimensions")
	flags.IntVar(&settings.SquareSize, "square-size", 512, "square output width and height")
	flags.IntVar(&settings.WideWidth, "wide-width", 768, "4:3 output width, a multiple of four")
	flags.StringVar(&settings.EncoderDirectory, "encoder-directory", "", "directory containing cwebp and avifenc; defaults to the executable checkout or PATH")
	flags.StringVar(&settings.Address, "address", "127.0.0.1:4179", "loopback listen address")
	version := flags.Bool("version", false, "print version and exit")
	if err := flags.Parse(args); err != nil {
		return editor.Config{}, err
	}
	if *version {
		fmt.Fprintln(output, "fabricum/"+editor.Version)
		return editor.Config{}, flag.ErrHelp
	}
	if flags.NArg() != 0 {
		return editor.Config{}, fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	if *configFile != "" {
		file, err := os.Open(*configFile)
		if err != nil {
			return editor.Config{}, err
		}
		defer file.Close()
		decoder := json.NewDecoder(io.LimitReader(file, 64<<10))
		decoder.DisallowUnknownFields()
		var loaded fileConfig
		if err := decoder.Decode(&loaded); err != nil {
			return editor.Config{}, fmt.Errorf("decode config: %w", err)
		}
		if err := ensureJSONEnd(decoder); err != nil {
			return editor.Config{}, err
		}
		root, err := filepath.Abs(filepath.Dir(*configFile))
		if err != nil {
			return editor.Config{}, err
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
			return editor.Config{}, err
		}
	}
	mode := editor.ModeGUI
	if settings.Source != "" || cliPathFlagProvided(flags) {
		mode = editor.ModeCLI
	}
	options := editor.Config{Mode: mode, Address: settings.Address, Source: settings.Source, SourceSize: settings.SourceSize,
		SquareSize: settings.SquareSize, WideWidth: settings.WideWidth, OutputDirectory: settings.OutputDirectory,
		SquareOutput: settings.SquareOutput, WideOutput: settings.WideOutput, EncoderDirectory: settings.EncoderDirectory}
	if settings.Sources != nil {
		options.Sources = func() ([]editor.Source, error) { return settings.Sources, nil }
	}
	if len(settings.ExportCommand) > 0 {
		options.AfterExport = exportCommand(settings.ExportCommand, commandDirectory)
	}
	if err := editor.ValidateConfig(options); err != nil {
		return editor.Config{}, err
	}
	return options, nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err == io.EOF {
		return nil
	} else if err != nil {
		return fmt.Errorf("invalid trailing JSON: %w", err)
	}
	return fmt.Errorf("request must contain one JSON value")
}

func cliPathFlagProvided(flags *flag.FlagSet) bool {
	provided := false
	flags.Visit(func(option *flag.Flag) {
		switch option.Name {
		case "source", "s", "output", "o":
			provided = true
		}
	})
	return provided
}
