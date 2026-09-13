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
type TextureEncodingOptions = processing.TextureEncodingOptions
type TextureOutputSpec = processing.TextureOutputSpec
type TextureSetRequest = processing.TextureSetRequest
type KTX2EncodingOptions = processing.KTX2EncodingOptions
type KTX2OutputSpec = processing.KTX2OutputSpec
type KTX2SetRequest = processing.KTX2SetRequest
type MaterialSetRequest = processing.MaterialSetRequest
type OutputMeasurement = processing.OutputMeasurement
type ProcessedOutput = processing.ProcessedOutput
type ExportReceipt = cli.ExportReceipt
type TransformReceipt = cli.TransformReceipt
type TextureReceipt = cli.TextureReceipt
type AssetFacts = processing.AssetFacts
type InspectionReport = processing.InspectionReport

const InspectionSchemaVersion = processing.InspectionSchemaVersion
const ExportReceiptSchemaVersion = cli.ExportReceiptSchemaVersion
const TransformReceiptSchemaVersion = cli.TransformReceiptSchemaVersion
const TextureReceiptSchemaVersion = cli.TextureReceiptSchemaVersion

const (
	TextureEncodingETC1S     = processing.TextureEncodingETC1S
	TextureEncodingUASTCZstd = processing.TextureEncodingUASTCZstd
	TextureTransferLinear    = processing.TextureTransferLinear
	TextureTransferSRGB      = processing.TextureTransferSRGB
	TexturePrimariesBT709    = processing.TexturePrimariesBT709
)

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

// BuildTextureSet prepares and atomically writes a loose KTX2 texture set.
func BuildTextureSet(ctx context.Context, request TextureSetRequest) ([]OutputMeasurement, error) {
	return processing.BuildTextureSet(ctx, request)
}

// PrepareTextureSetOutputs prepares a loose KTX2 texture set without writing
// its output files.
func PrepareTextureSetOutputs(ctx context.Context, request TextureSetRequest) ([]ProcessedOutput, error) {
	return processing.PrepareTextureSetOutputs(ctx, request)
}

// EncodeTexture prepares one standalone loose KTX2 output without writing it.
func EncodeTexture(ctx context.Context, request TextureOutputSpec, encoderDirectory string) (ProcessedOutput, error) {
	return processing.EncodeTexture(ctx, request, encoderDirectory)
}

// TransformFile reads a JSON transform request and writes its versioned
// receipt to output.
func TransformFile(ctx context.Context, path string, output io.Writer) error {
	return cli.RunTransformFile(ctx, path, output)
}

// TextureSetFile reads a JSON texture-set request and writes its versioned
// receipt to output.
func TextureSetFile(ctx context.Context, path string, output io.Writer) error {
	return cli.RunTextureSetFile(ctx, path, output)
}

// Inspect writes a versioned, machine-readable report for the supplied files.
// The report is written even when one or more individual files fail.
func Inspect(paths []string, output io.Writer) error {
	return cli.RunInspection(paths, output)
}
