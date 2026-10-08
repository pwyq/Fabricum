package processing

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
)

var optimizationPNGSignature = []byte("\x89PNG\r\n\x1a\n")

type optimizationPNGChunk struct {
	kind      string
	data, raw []byte
}

type optimizationPNG struct {
	chunks []optimizationPNGChunk
	pixels image.Image
}

func decodeOptimizationPNG(data []byte) (optimizationPNG, error) {
	chunks, err := readOptimizationPNGChunks(data)
	if err != nil {
		return optimizationPNG{}, err
	}
	header := chunks[0].data
	width, height := binary.BigEndian.Uint32(header), binary.BigEndian.Uint32(header[4:])
	if width == 0 || height == 0 || uint64(width)*uint64(height) > maxInspectionPixels {
		return optimizationPNG{}, fmt.Errorf("PNG exceeds %d pixel limit or has invalid dimensions", maxInspectionPixels)
	}
	pixels, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return optimizationPNG{}, err
	}
	return optimizationPNG{chunks: chunks, pixels: pixels}, nil
}

func readOptimizationPNGChunks(data []byte) ([]optimizationPNGChunk, error) {
	if !bytes.HasPrefix(data, optimizationPNGSignature) {
		return nil, fmt.Errorf("missing PNG signature")
	}
	var chunks []optimizationPNGChunk
	seen := make(map[string]bool)
	idatEnded := false
	for offset := 8; offset < len(data); {
		if len(data)-offset < 12 {
			return nil, fmt.Errorf("truncated PNG chunk")
		}
		length := uint64(binary.BigEndian.Uint32(data[offset:]))
		if length > uint64(len(data)-offset-12) {
			return nil, fmt.Errorf("truncated PNG chunk data")
		}
		end := offset + int(length) + 12
		raw := data[offset:end]
		kind := string(raw[4:8])
		for i, c := range raw[4:8] {
			if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) || (i == 2 && c >= 'a') {
				return nil, fmt.Errorf("invalid PNG chunk name %q", kind)
			}
		}
		if crc32.ChecksumIEEE(raw[4:len(raw)-4]) != binary.BigEndian.Uint32(raw[len(raw)-4:]) {
			return nil, fmt.Errorf("invalid %s checksum", kind)
		}
		if len(chunks) == 0 && (kind != "IHDR" || length != 13) {
			return nil, fmt.Errorf("PNG must start with a 13-byte IHDR")
		}
		if kind != "IDAT" && seen["IDAT"] {
			idatEnded = true
		}
		if kind == "IDAT" && idatEnded {
			return nil, fmt.Errorf("PNG IDAT chunks must be consecutive")
		}
		switch kind {
		case "IHDR", "PLTE", "tRNS", "IEND":
			if seen[kind] {
				return nil, fmt.Errorf("duplicate %s chunk", kind)
			}
		case "IDAT":
		case "acTL", "fcTL", "fdAT", "dSIG", "caBX", "iDOT", "CgBI":
			return nil, fmt.Errorf("unsupported %s chunk: animation or byte-dependent PNG data", kind)
		case "cHRM", "gAMA", "iCCP", "sBIT", "sRGB", "bKGD", "hIST", "pHYs", "sPLT", "tIME", "iTXt", "tEXt", "zTXt", "eXIf", "cICP", "mDCV", "cLLI":
		default:
			if raw[4] < 'a' || raw[7] < 'a' {
				return nil, fmt.Errorf("unsupported PNG chunk %s is critical or unsafe to copy", kind)
			}
		}
		seen[kind] = true
		chunks = append(chunks, optimizationPNGChunk{kind: kind, data: raw[8 : len(raw)-4], raw: raw})
		offset = end
		if kind == "IEND" {
			if length != 0 || offset != len(data) || !seen["IDAT"] {
				return nil, fmt.Errorf("invalid IEND or trailing PNG data")
			}
			return chunks, nil
		}
	}
	return nil, fmt.Errorf("missing PNG IEND")
}

func rebuildOptimizedPNG(original, optimized []optimizationPNGChunk, strip bool) ([]byte, bool) {
	output := append([]byte(nil), optimizationPNGSignature...)
	inserted, stripped := false, false
	for _, chunk := range original {
		if chunk.kind == "IDAT" {
			if !inserted {
				for _, candidate := range optimized {
					if candidate.kind == "IDAT" {
						output = append(output, candidate.raw...)
					}
				}
				inserted = true
			}
			continue
		}
		if strip && (chunk.kind == "tEXt" || chunk.kind == "zTXt" || chunk.kind == "iTXt" || chunk.kind == "tIME") {
			stripped = true
			continue
		}
		output = append(output, chunk.raw...)
	}
	return output, stripped
}

func samePNGPixelValues(ctx context.Context, original, candidate image.Image) error {
	if original.Bounds() != candidate.Bounds() {
		return fmt.Errorf("dimensions changed")
	}
	for y := original.Bounds().Min.Y; y < original.Bounds().Max.Y; y++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		for x := original.Bounds().Min.X; x < original.Bounds().Max.X; x++ {
			a := unpremultipliedPNGValue(original.At(x, y))
			b := unpremultipliedPNGValue(candidate.At(x, y))
			if a != b {
				return fmt.Errorf("RGBA values changed at (%d, %d)", x, y)
			}
		}
	}
	return nil
}

func unpremultipliedPNGValue(pixel color.Color) color.NRGBA64 {
	switch value := pixel.(type) {
	case color.NRGBA:
		// Generic model conversion goes through RGBA(), which premultiplies and
		// loses RGB when alpha is zero. Expand straight samples directly.
		return color.NRGBA64{R: uint16(value.R) * 257, G: uint16(value.G) * 257, B: uint16(value.B) * 257, A: uint16(value.A) * 257}
	case color.NRGBA64:
		return value
	default:
		return color.NRGBA64Model.Convert(pixel).(color.NRGBA64)
	}
}
