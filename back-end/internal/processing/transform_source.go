package processing

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"os"
	"strings"
)

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
