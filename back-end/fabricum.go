// Package fabricum exposes the stable host interface for the local artwork editor.
package fabricum

import (
	"io"
	"net/http"

	"fabricum/back-end/internal/cli"
	"fabricum/back-end/internal/editor"
	"fabricum/back-end/internal/processing"
)

const (
	Version = editor.Version
	ModeGUI = editor.ModeGUI
	ModeCLI = editor.ModeCLI
)

type Source = editor.Source
type Config = editor.Config
type ExportRequest = processing.ExportRequest
type CropRect = processing.CropRect
type OutputMeasurement = processing.OutputMeasurement
type ExportReceipt = cli.ExportReceipt
type AssetFacts = processing.AssetFacts
type InspectionReport = processing.InspectionReport

const InspectionSchemaVersion = processing.InspectionSchemaVersion
const ExportReceiptSchemaVersion = cli.ExportReceiptSchemaVersion

func ParseConfig(args []string, output io.Writer) (Config, error) {
	return cli.ParseConfig(args, output)
}

func NewHandler(options Config) (http.Handler, error) {
	return editor.NewHandler(options)
}

func Serve(options Config) error {
	return editor.Serve(options)
}

// WriteFileAtomically replaces one file after synchronizing its temporary file.
// An export of multiple files is not a transaction.
func WriteFileAtomically(path string, data []byte) error {
	return processing.WriteFileAtomically(path, data)
}

// Inspect writes a versioned, machine-readable report for the supplied files.
// The report is written even when one or more individual files fail.
func Inspect(paths []string, output io.Writer) error {
	return cli.RunInspection(paths, output)
}
