package processing

import (
	"crypto/sha256"
	"encoding/hex"
	"image"
	"path/filepath"
)

func newOutputMeasurement(role, path string, output image.Image, format, pngMode string, quality int, lossless bool, filter string, crop CropRect, data []byte) OutputMeasurement {
	hash := sha256Bytes(data)
	return OutputMeasurement{
		Role: role, Path: filepath.ToSlash(path), Width: output.Bounds().Dx(), Height: output.Bounds().Dy(),
		Format: format, PNGMode: pngMode, Encoder: EncoderForFormat(format), EncoderVersion: EncoderVersionForFormat(format),
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
