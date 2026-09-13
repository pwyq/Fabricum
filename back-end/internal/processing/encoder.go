package processing

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"path/filepath"
	"strings"
)

func validateEncodingOptions(request ExportRequest) error {
	if request.Format != "png" && request.Format != "webp" && request.Format != "avif" {
		return errors.New("format must be png, webp, or avif")
	}
	if request.Format == "png" {
		if request.Quality != 0 || request.Lossless {
			return errors.New("PNG does not accept quality or lossless options")
		}
		return nil
	}
	if request.Quality < 1 || request.Quality > 100 {
		return errors.New("quality must be between 1 and 100")
	}
	return nil
}

func OutputPath(path, format string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + "." + format
}

func encodeOutput(ctx context.Context, source image.Image, request ExportRequest, encoderDirectory string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	compression := png.BestCompression
	if request.Format != "png" {
		compression = png.BestSpeed
	}
	pngInput, err := encodePNG(source, compression)
	if err != nil {
		return nil, err
	}
	if request.Format == "png" {
		return pngInput, nil
	}
	return encodeNative(ctx, pngInput, request, encoderDirectory)
}

func encodePNG(source image.Image, compression png.CompressionLevel) ([]byte, error) {
	var output bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: compression}).Encode(&output, source); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
