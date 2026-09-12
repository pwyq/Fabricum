package fabricum

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed encoder/encode.mjs
var encoderScript string

func validateEncodingOptions(request exportRequest) error {
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

func outputPath(path, format string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + "." + format
}

func encodeOutput(ctx context.Context, source image.Image, request exportRequest, encoderDirectory string) ([]byte, error) {
	var pngInput bytes.Buffer
	compression := png.BestCompression
	if request.Format != "png" {
		compression = png.BestSpeed
	}
	if err := (&png.Encoder{CompressionLevel: compression}).Encode(&pngInput, source); err != nil {
		return nil, err
	}
	if request.Format == "png" {
		return pngInput.Bytes(), nil
	}
	command := exec.CommandContext(ctx, "node", "--input-type=module", "-e", encoderScript, "--", request.Format, strconv.Itoa(request.Quality), strconv.FormatBool(request.Lossless))
	command.Dir = encoderDirectory
	command.Stdin = &pngInput
	var output, stderr bytes.Buffer
	command.Stdout, command.Stderr = &output, &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("run Sharp encoder: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return output.Bytes(), nil
}
