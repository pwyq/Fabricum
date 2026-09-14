package processing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxTransformOutputs = 1024
	maxTransformPath    = 4096
	maxTransformPixels  = 64 << 20
)

// Transform prepares and atomically writes every output in request. No output
// is written until all source validation, transformations, and encoding have
// succeeded.
func Transform(ctx context.Context, request TransformRequest) ([]OutputMeasurement, error) {
	outputs, err := PrepareTransformOutputs(ctx, request)
	if err != nil {
		return nil, err
	}
	if err := WriteOutputs(outputs); err != nil {
		return nil, err
	}
	return OutputMeasurements(outputs), nil
}

// PrepareTransformOutputs decodes and encodes a transform request without
// writing its outputs. It is useful for previews and for callers that need to
// inspect output bytes before committing them.
func PrepareTransformOutputs(ctx context.Context, request TransformRequest) ([]ProcessedOutput, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	request.Format = strings.ToLower(strings.TrimSpace(request.Format))
	if request.Format == "" {
		request.Format = "png"
	}
	if err := validateTransformRequest(request); err != nil {
		return nil, err
	}
	if err := validateTransformPaths(request); err != nil {
		return nil, err
	}
	primary, _, err := readTransformImage(request.Source, request.effectiveConstraints())
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	outputs := make([]ProcessedOutput, 0, len(request.Outputs))
	for index, spec := range request.Outputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		imageOutput, filter, err := renderTransform(primary, spec.Transform)
		if err != nil {
			return nil, fmt.Errorf("output %s: %w", outputRole(spec, index), err)
		}
		data, err := encodeOutput(ctx, imageOutput, ExportRequest{
			Format: request.Format, Quality: request.Quality, Lossless: request.Lossless,
		}, request.EncoderDirectory)
		if err != nil {
			return nil, fmt.Errorf("encode %s output: %w", outputRole(spec, index), err)
		}
		path := OutputPath(spec.Path, request.Format)
		crop := CropRect{}
		if spec.Transform.Crop != nil {
			crop = *spec.Transform.Crop
		}
		outputs = append(outputs, ProcessedOutput{
			Path:        path,
			Data:        data,
			Measurement: newOutputMeasurement(outputRole(spec, index), path, imageOutput, request.Format, request.Quality, request.Lossless, filter, crop, data),
		})
	}
	return outputs, nil
}

func (request TransformRequest) effectiveConstraints() SourceConstraints {
	if request.SourceConstraints != nil {
		return *request.SourceConstraints
	}
	return request.Constraints
}

func validateTransformRequest(request TransformRequest) error {
	if request.Source == "" || len(request.Source) > maxTransformPath {
		return errors.New("source path must contain between 1 and 4096 bytes")
	}
	if len(request.Outputs) == 0 {
		return errors.New("at least one transform output is required")
	}
	if len(request.Outputs) > maxTransformOutputs {
		return fmt.Errorf("at most %d transform outputs are allowed", maxTransformOutputs)
	}
	if err := validateEncodingOptions(ExportRequest{Format: request.Format, Quality: request.Quality, Lossless: request.Lossless}); err != nil {
		return err
	}
	constraints := request.effectiveConstraints()
	if constraints.Width < 0 || constraints.Height < 0 || (constraints.Width == 0) != (constraints.Height == 0) {
		return errors.New("source width and height must both be positive or both be zero")
	}
	if constraints.Format != "" {
		if _, ok := normalizeImageFormat(constraints.Format); !ok {
			return fmt.Errorf("unsupported source format %q", constraints.Format)
		}
	}
	for index, output := range request.Outputs {
		if output.Path == "" || len(output.Path) > maxTransformPath {
			return fmt.Errorf("output %d path must contain between 1 and 4096 bytes", index+1)
		}
		if err := validateImageTransform(output.Transform); err != nil {
			return fmt.Errorf("output %s: %w", outputRole(output, index), err)
		}
	}
	return nil
}

func validateTransformPaths(request TransformRequest) error {
	source, err := filepath.Abs(request.Source)
	if err != nil {
		return fmt.Errorf("resolve source path: %w", err)
	}
	paths := make([]string, 0, len(request.Outputs))
	for index, output := range request.Outputs {
		path := OutputPath(output.Path, request.Format)
		absolute, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve %s output path: %w", outputRole(output, index), err)
		}
		if sameTransformPath(source, absolute) {
			return errors.New("source and transform output paths must be distinct")
		}
		for _, existing := range paths {
			if sameTransformPath(existing, absolute) {
				return errors.New("transform output paths must be distinct")
			}
		}
		paths = append(paths, absolute)
	}
	return nil
}

func sameTransformPath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtimeWindows() {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func runtimeWindows() bool {
	return filepath.Separator == '\\'
}

func outputRole(output TransformOutputSpec, index int) string {
	if output.Role != "" {
		return output.Role
	}
	return fmt.Sprintf("output-%d", index+1)
}

func validateImageTransform(transform ImageTransform) error {
	if transform.Crop != nil {
		if transform.Crop.Width <= 0 || transform.Crop.Height <= 0 || transform.Crop.X < 0 || transform.Crop.Y < 0 {
			return errors.New("crop coordinates and dimensions must be non-negative, with positive dimensions")
		}
	}
	if transform.Resize != nil {
		if err := validateResizeSpec(*transform.Resize); err != nil {
			return err
		}
	}
	if transform.Padding != nil {
		padding := transform.Padding
		if padding.Top < 0 || padding.Right < 0 || padding.Bottom < 0 || padding.Left < 0 {
			return errors.New("padding values cannot be negative")
		}
	}
	if transform.Channel != "" && !validChannel(transform.Channel) {
		return fmt.Errorf("unsupported channel %q", transform.Channel)
	}
	if transform.Pack != nil {
		for _, channel := range []struct {
			name  string
			input ChannelInput
		}{
			{name: "red", input: transform.Pack.Red},
			{name: "green", input: transform.Pack.Green},
			{name: "blue", input: transform.Pack.Blue},
		} {
			if err := validateChannelInput(channel.name, channel.input); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateResizeSpec(resize ResizeSpec) error {
	if resize.Width < 1 || resize.Height < 1 || resize.Width > 8192 || resize.Height > 8192 {
		return errors.New("resize dimensions must be between 1 and 8192")
	}
	if int64(resize.Width)*int64(resize.Height) > maxTransformPixels {
		return errors.New("resize dimensions exceed 67108864 pixels")
	}
	fit := strings.ToLower(strings.TrimSpace(resize.Fit))
	if fit != "" && fit != "fill" && fit != "contain" {
		return fmt.Errorf("resize fit must be fill or contain, got %q", resize.Fit)
	}
	if _, ok := normalizeFilter(resize.Filter); !ok {
		return fmt.Errorf("unsupported resize filter %q", resize.Filter)
	}
	return nil
}

func validateChannelInput(name string, input ChannelInput) error {
	if input.Constant != nil {
		if input.Source != "" || input.Channel != "" {
			return fmt.Errorf("%s channel constant cannot include a source or channel", name)
		}
		return nil
	}
	if input.Channel != "" && !validChannel(input.Channel) {
		return fmt.Errorf("unsupported %s channel %q", name, input.Channel)
	}
	if input.Source != "" && len(input.Source) > maxTransformPath {
		return fmt.Errorf("%s channel source path is too long", name)
	}
	return nil
}

func readTransformImage(path string, constraints SourceConstraints) (image.Image, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", fmt.Errorf("read source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, "", errors.New("source is not a regular file")
	}
	if info.Size() > maxInspectionFileBytes {
		return nil, "", fmt.Errorf("source exceeds %d byte limit", maxInspectionFileBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read source: %w", err)
	}
	format := detectedFormat(data, "")
	if format != "png" && format != "jpeg" && format != "gif" {
		return nil, "", fmt.Errorf("source format %q cannot be decoded by the built-in image processor", actualFormatName(format))
	}
	facts, err := inspectByFormat(path, data, format)
	if err != nil {
		return nil, "", err
	}
	if expected, ok := normalizeImageFormat(constraints.Format); ok && expected != format {
		return nil, "", fmt.Errorf("source format must be %s, got %s", expected, format)
	}
	if constraints.Format != "" {
		if _, ok := normalizeImageFormat(constraints.Format); !ok {
			return nil, "", fmt.Errorf("unsupported source format %q", constraints.Format)
		}
	}
	if constraints.HasAlpha != nil && facts.HasAlpha != *constraints.HasAlpha {
		return nil, "", fmt.Errorf("source alpha presence must be %t", *constraints.HasAlpha)
	}
	if constraints.Width > 0 && (facts.Width != constraints.Width || facts.Height != constraints.Height) {
		return nil, "", fmt.Errorf("source must be exactly %dx%d, got %dx%d", constraints.Width, constraints.Height, facts.Width, facts.Height)
	}
	if constraints.SingleFrame && format == "gif" {
		animation, err := gif.DecodeAll(bytes.NewReader(data))
		if err != nil {
			return nil, "", fmt.Errorf("decode GIF frames: %w", err)
		}
		if len(animation.Image) != 1 {
			return nil, "", fmt.Errorf("source must contain one frame, got %d", len(animation.Image))
		}
	}
	decoded, decodedFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("decode source: %w", err)
	}
	if decodedFormat != format {
		return nil, "", fmt.Errorf("decoded source format %s does not match detected %s", decodedFormat, format)
	}
	return decoded, format, nil
}

func normalizeImageFormat(format string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "png":
		return "png", true
	case "jpg", "jpeg":
		return "jpeg", true
	case "gif":
		return "gif", true
	default:
		return "", false
	}
}

func renderTransform(primary image.Image, transform ImageTransform) (image.Image, string, error) {
	spatial, filter, err := applySpatialTransform(primary, transform)
	if err != nil {
		return nil, "", err
	}
	if transform.Pack != nil {
		packed, err := packChannels(spatial, transform)
		return packed, filter, err
	}
	return applyChannelTransform(spatial, transform), filter, nil
}

func applySpatialTransform(source image.Image, transform ImageTransform) (image.Image, string, error) {
	current := source
	if transform.Crop != nil {
		rect, err := cropRectangle(source.Bounds(), *transform.Crop)
		if err != nil {
			return nil, "", err
		}
		current = copyImage(source, rect)
	}
	filter := ""
	if transform.Resize != nil {
		resize := *transform.Resize
		filter = effectiveFilter(resize.Filter)
		fit := strings.ToLower(strings.TrimSpace(resize.Fit))
		if fit == "" {
			fit = "fill"
		}
		if fit == "fill" {
			current = resizeImage(current, current.Bounds(), resize.Width, resize.Height, filter)
		} else {
			current = resizeContain(current, resize.Width, resize.Height, filter)
		}
	}
	if transform.Padding != nil {
		var err error
		current, err = padTransparent(current, *transform.Padding)
		if err != nil {
			return nil, filter, err
		}
	}
	if err := validateTransformDimensions(current.Bounds()); err != nil {
		return nil, filter, err
	}
	return current, filter, nil
}

func validateTransformDimensions(bounds image.Rectangle) error {
	if bounds.Dx() < 1 || bounds.Dy() < 1 || bounds.Dx() > 8192 || bounds.Dy() > 8192 || int64(bounds.Dx())*int64(bounds.Dy()) > maxTransformPixels {
		return errors.New("output dimensions exceed 8192 pixels per side or 67108864 pixels")
	}
	return nil
}

func cropRectangle(bounds image.Rectangle, crop CropRect) (image.Rectangle, error) {
	if crop.Width <= 0 || crop.Height <= 0 {
		return image.Rectangle{}, errors.New("crop dimensions must be positive")
	}
	if crop.X < 0 || crop.Y < 0 || crop.Width > bounds.Dx() || crop.Height > bounds.Dy() || crop.X > bounds.Dx()-crop.Width || crop.Y > bounds.Dy()-crop.Height {
		return image.Rectangle{}, fmt.Errorf("crop rectangle %+v exceeds %dx%d source", crop, bounds.Dx(), bounds.Dy())
	}
	return image.Rect(bounds.Min.X+crop.X, bounds.Min.Y+crop.Y, bounds.Min.X+crop.X+crop.Width, bounds.Min.Y+crop.Y+crop.Height), nil
}

func copyImage(source image.Image, rect image.Rectangle) *image.NRGBA {
	result := image.NewNRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	for y := 0; y < rect.Dy(); y++ {
		for x := 0; x < rect.Dx(); x++ {
			result.SetNRGBA(x, y, nrgbaAtAbsolute(source, rect.Min.X+x, rect.Min.Y+y))
		}
	}
	return result
}

func resizeContain(source image.Image, width, height int, filter string) image.Image {
	sourceWidth, sourceHeight := source.Bounds().Dx(), source.Bounds().Dy()
	var scaledWidth, scaledHeight int
	if int64(width)*int64(sourceHeight) <= int64(height)*int64(sourceWidth) {
		scaledWidth = width
		scaledHeight = roundedRatio(sourceHeight, width, sourceWidth)
	} else {
		scaledWidth = roundedRatio(sourceWidth, height, sourceHeight)
		scaledHeight = height
	}
	scaledWidth = max(1, min(width, scaledWidth))
	scaledHeight = max(1, min(height, scaledHeight))
	resized := resizeImage(source, source.Bounds(), scaledWidth, scaledHeight, filter)
	result := image.NewNRGBA(image.Rect(0, 0, width, height))
	offsetX, offsetY := (width-scaledWidth)/2, (height-scaledHeight)/2
	for y := 0; y < scaledHeight; y++ {
		for x := 0; x < scaledWidth; x++ {
			result.SetNRGBA(offsetX+x, offsetY+y, nrgbaAt(resized, x, y))
		}
	}
	return result
}

func roundedRatio(value, numerator, denominator int) int {
	return int((int64(value)*int64(numerator) + int64(denominator)/2) / int64(denominator))
}

func padTransparent(source image.Image, padding PaddingSpec) (image.Image, error) {
	width := int64(source.Bounds().Dx()) + int64(padding.Left) + int64(padding.Right)
	height := int64(source.Bounds().Dy()) + int64(padding.Top) + int64(padding.Bottom)
	if width < 1 || height < 1 || width > 8192 || height > 8192 || width*height > maxTransformPixels {
		return nil, errors.New("padded dimensions exceed output limits")
	}
	result := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)))
	for y := 0; y < source.Bounds().Dy(); y++ {
		for x := 0; x < source.Bounds().Dx(); x++ {
			result.SetNRGBA(padding.Left+x, padding.Top+y, nrgbaAtAbsolute(source, source.Bounds().Min.X+x, source.Bounds().Min.Y+y))
		}
	}
	return result, nil
}

func applyChannelTransform(source image.Image, transform ImageTransform) image.Image {
	if transform.Channel != "" {
		result := image.NewGray(image.Rect(0, 0, source.Bounds().Dx(), source.Bounds().Dy()))
		for y := 0; y < result.Bounds().Dy(); y++ {
			for x := 0; x < result.Bounds().Dx(); x++ {
				result.SetGray(x, y, colorGray(channelValue(nrgbaAtAbsolute(source, source.Bounds().Min.X+x, source.Bounds().Min.Y+y), transform.Channel)))
			}
		}
		return result
	}
	result := copyImage(source, source.Bounds())
	if transform.Grayscale {
		for y := 0; y < result.Bounds().Dy(); y++ {
			for x := 0; x < result.Bounds().Dx(); x++ {
				pixel := result.NRGBAAt(x, y)
				gray := grayscaleValue(pixel)
				result.SetNRGBA(x, y, color.NRGBA{R: gray, G: gray, B: gray, A: pixel.A})
			}
		}
	}
	if transform.RemoveAlpha {
		for y := 0; y < result.Bounds().Dy(); y++ {
			for x := 0; x < result.Bounds().Dx(); x++ {
				pixel := result.NRGBAAt(x, y)
				pixel.A = 255
				result.SetNRGBA(x, y, pixel)
			}
		}
	}
	return result
}

func packChannels(primarySpatial image.Image, transform ImageTransform) (image.Image, error) {
	pack := transform.Pack
	result := image.NewNRGBA(image.Rect(0, 0, primarySpatial.Bounds().Dx(), primarySpatial.Bounds().Dy()))
	for y := 0; y < result.Bounds().Dy(); y++ {
		for x := 0; x < result.Bounds().Dx(); x++ {
			result.SetNRGBA(x, y, color.NRGBA{A: 255})
		}
	}
	inputs := []struct {
		name  string
		input ChannelInput
		set   func(*color.NRGBA, uint8)
	}{
		{name: "red", input: pack.Red, set: func(pixel *color.NRGBA, value uint8) { pixel.R = value }},
		{name: "green", input: pack.Green, set: func(pixel *color.NRGBA, value uint8) { pixel.G = value }},
		{name: "blue", input: pack.Blue, set: func(pixel *color.NRGBA, value uint8) { pixel.B = value }},
	}
	for _, entry := range inputs {
		if entry.input.Constant != nil {
			for y := 0; y < result.Bounds().Dy(); y++ {
				for x := 0; x < result.Bounds().Dx(); x++ {
					pixel := result.NRGBAAt(x, y)
					entry.set(&pixel, *entry.input.Constant)
					result.SetNRGBA(x, y, pixel)
				}
			}
			continue
		}
		channelSource := primarySpatial
		if entry.input.Source != "" {
			decoded, _, err := readTransformImage(entry.input.Source, SourceConstraints{})
			if err != nil {
				return nil, fmt.Errorf("%s channel: %w", entry.name, err)
			}
			channelSource, _, err = applySpatialTransform(decoded, transform)
			if err != nil {
				return nil, fmt.Errorf("%s channel: %w", entry.name, err)
			}
		}
		if channelSource.Bounds().Dx() != result.Bounds().Dx() || channelSource.Bounds().Dy() != result.Bounds().Dy() {
			return nil, fmt.Errorf("%s channel dimensions are %dx%d, want %dx%d", entry.name, channelSource.Bounds().Dx(), channelSource.Bounds().Dy(), result.Bounds().Dx(), result.Bounds().Dy())
		}
		channel := entry.input.Channel
		if channel == "" {
			channel = "gray"
		}
		for y := 0; y < result.Bounds().Dy(); y++ {
			for x := 0; x < result.Bounds().Dx(); x++ {
				pixel := result.NRGBAAt(x, y)
				entry.set(&pixel, channelValue(nrgbaAtAbsolute(channelSource, channelSource.Bounds().Min.X+x, channelSource.Bounds().Min.Y+y), channel))
				result.SetNRGBA(x, y, pixel)
			}
		}
	}
	return result, nil
}

func validChannel(channel string) bool {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "r", "red", "g", "green", "b", "blue", "a", "alpha", "gray", "grey", "luma", "luminance":
		return true
	default:
		return false
	}
}

func channelValue(pixel color.NRGBA, channel string) uint8 {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "r", "red":
		return pixel.R
	case "g", "green":
		return pixel.G
	case "b", "blue":
		return pixel.B
	case "a", "alpha":
		return pixel.A
	default:
		return grayscaleValue(pixel)
	}
}

func grayscaleValue(pixel color.NRGBA) uint8 {
	return uint8((299*uint32(pixel.R) + 587*uint32(pixel.G) + 114*uint32(pixel.B) + 500) / 1000)
}

func colorGray(value uint8) color.Gray {
	return color.Gray{Y: value}
}

func newOutputMeasurement(role, path string, output image.Image, format string, quality int, lossless bool, filter string, crop CropRect, data []byte) OutputMeasurement {
	hash := sha256Bytes(data)
	return OutputMeasurement{
		Role: role, Path: filepath.ToSlash(path), Width: output.Bounds().Dx(), Height: output.Bounds().Dy(),
		Format: format, Encoder: EncoderForFormat(format), EncoderVersion: EncoderVersionForFormat(format),
		Processor: ProcessorName, NativeEncoderVersions: EncoderVersionsForFormat(format), HasAlpha: !imageOpaque(output),
		Filter: filter, Quality: quality, Lossless: lossless, Bytes: len(data), SHA256: hash, Crop: crop,
	}
}

func imageOpaque(source image.Image) bool {
	if opaque, ok := source.(interface{ Opaque() bool }); ok {
		return opaque.Opaque()
	}
	bounds := source.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := source.At(x, y).RGBA()
			if alpha != 0xffff {
				return false
			}
		}
	}
	return true
}

func sha256Bytes(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
