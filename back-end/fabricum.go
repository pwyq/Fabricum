// Package fabricum exposes the stable host interface for the local artwork editor.
package fabricum

import (
	"context"
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
type SourceConstraints = processing.SourceConstraints
type ResizeSpec = processing.ResizeSpec
type PaddingSpec = processing.PaddingSpec
type ChannelInput = processing.ChannelInput
type ChannelPack = processing.ChannelPack
type ImageTransform = processing.ImageTransform
type TransformOutputSpec = processing.TransformOutputSpec
type TransformRequest = processing.TransformRequest
type OutputMeasurement = processing.OutputMeasurement
type ProcessedOutput = processing.ProcessedOutput
type ExportReceipt = cli.ExportReceipt
type TransformReceipt = cli.TransformReceipt
type AssetFacts = processing.AssetFacts
type InspectionReport = processing.InspectionReport

const InspectionSchemaVersion = processing.InspectionSchemaVersion
const ExportReceiptSchemaVersion = cli.ExportReceiptSchemaVersion
const TransformReceiptSchemaVersion = cli.TransformReceiptSchemaVersion

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

// Transform prepares and atomically writes a noninteractive transform.
func Transform(ctx context.Context, request TransformRequest) ([]OutputMeasurement, error) {
	return processing.Transform(ctx, request)
}

// PrepareTransformOutputs prepares a noninteractive transform without writing
// its output files.
func PrepareTransformOutputs(ctx context.Context, request TransformRequest) ([]ProcessedOutput, error) {
	return processing.PrepareTransformOutputs(ctx, request)
}

// TransformFile reads a JSON transform request and writes its versioned
// receipt to output.
func TransformFile(ctx context.Context, path string, output io.Writer) error {
	return cli.RunTransformFile(ctx, path, output)
}

// Inspect writes a versioned, machine-readable report for the supplied files.
// The report is written even when one or more individual files fail.
func Inspect(paths []string, output io.Writer) error {
	return cli.RunInspection(paths, output)
}
