package processing

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image/gif"
	"image/jpeg"
	"image/png"
)

var pngMagic = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

func isPNG(data []byte) bool {
	return len(data) >= len(pngMagic) && bytes.Equal(data[:len(pngMagic)], pngMagic)
}

func isJPEG(data []byte) bool {
	return len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff
}

func isGIF(data []byte) bool {
	return len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a")))
}

func isWebP(data []byte) bool {
	return len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP"))
}

func inspectPNG(data []byte) (AssetFacts, error) {
	width, height, hasAlpha, err := parsePNGHeader(data)
	if err != nil {
		return AssetFacts{}, err
	}
	if _, err := png.DecodeConfig(bytes.NewReader(data)); err != nil {
		return AssetFacts{}, fmt.Errorf("decode PNG: %w", err)
	}
	return AssetFacts{Width: width, Height: height, HasAlpha: hasAlpha}, nil
}

func parsePNGHeader(data []byte) (int, int, bool, error) {
	if !isPNG(data) {
		return 0, 0, false, errors.New("invalid PNG signature")
	}
	var width, height int
	var hasAlpha, sawHeader, sawData, sawEnd bool
	for offset := len(pngMagic); offset < len(data); {
		if len(data)-offset < 12 {
			return 0, 0, false, errors.New("truncated PNG chunk")
		}
		length64 := uint64(binary.BigEndian.Uint32(data[offset : offset+4]))
		if length64 > uint64(len(data)-offset-12) {
			return 0, 0, false, errors.New("truncated PNG chunk data")
		}
		length := int(length64)
		name := string(data[offset+4 : offset+8])
		chunk := data[offset+8 : offset+8+length]
		switch name {
		case "IHDR":
			if sawHeader || length != 13 || offset != len(pngMagic) {
				return 0, 0, false, errors.New("invalid PNG header")
			}
			width = int(binary.BigEndian.Uint32(chunk[:4]))
			height = int(binary.BigEndian.Uint32(chunk[4:8]))
			if width < 1 || height < 1 || int64(width)*int64(height) > maxInspectionPixels {
				return 0, 0, false, errors.New("PNG dimensions exceed inspection limits")
			}
			if !validPNGColorType(chunk[9]) {
				return 0, 0, false, errors.New("unsupported PNG color type")
			}
			hasAlpha = chunk[9] == 4 || chunk[9] == 6
			sawHeader = true
		case "tRNS":
			hasAlpha = true
		case "IDAT":
			sawData = true
		case "IEND":
			if length != 0 || sawEnd {
				return 0, 0, false, errors.New("invalid PNG end marker")
			}
			sawEnd = true
			if offset+12 != len(data) {
				return 0, 0, false, errors.New("data follows PNG end marker")
			}
		}
		offset += 12 + length
	}
	if !sawHeader || !sawData || !sawEnd {
		return 0, 0, false, errors.New("incomplete PNG")
	}
	return width, height, hasAlpha, nil
}

func validPNGColorType(colorType byte) bool {
	switch colorType {
	case 0, 2, 3, 4, 6:
		return true
	default:
		return false
	}
}

func inspectJPEG(data []byte) (AssetFacts, error) {
	if !isJPEG(data) {
		return AssetFacts{}, errors.New("invalid JPEG signature")
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return AssetFacts{}, fmt.Errorf("decode JPEG: %w", err)
	}
	if err := validateImageDimensions(config.Width, config.Height); err != nil {
		return AssetFacts{}, fmt.Errorf("JPEG %w", err)
	}
	return AssetFacts{Width: config.Width, Height: config.Height}, nil
}

func inspectGIF(data []byte) (AssetFacts, error) {
	if !isGIF(data) {
		return AssetFacts{}, errors.New("invalid GIF signature")
	}
	config, err := gif.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return AssetFacts{}, fmt.Errorf("decode GIF: %w", err)
	}
	if err := validateImageDimensions(config.Width, config.Height); err != nil {
		return AssetFacts{}, fmt.Errorf("GIF %w", err)
	}
	return AssetFacts{Width: config.Width, Height: config.Height, HasAlpha: gifHasTransparency(data)}, nil
}

func gifHasTransparency(data []byte) bool {
	for offset := 0; offset+7 <= len(data); offset++ {
		if data[offset] == 0x21 && data[offset+1] == 0xf9 && data[offset+2] == 0x04 && data[offset+3]&0x01 != 0 {
			return true
		}
	}
	return false
}

func inspectWebP(data []byte) (AssetFacts, error) {
	if !isWebP(data) || uint64(binary.LittleEndian.Uint32(data[4:8]))+8 != uint64(len(data)) {
		return AssetFacts{}, errors.New("invalid WebP RIFF container")
	}
	var width, height int
	var hasAlpha, sawImage bool
	for offset := 12; offset < len(data); {
		if len(data)-offset < 8 {
			return AssetFacts{}, errors.New("truncated WebP chunk")
		}
		length := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		if length < 0 || length > len(data)-offset-8 {
			return AssetFacts{}, errors.New("truncated WebP chunk data")
		}
		chunk := data[offset+8 : offset+8+length]
		switch string(data[offset : offset+4]) {
		case "VP8X":
			var err error
			width, height, err = parseVP8X(chunk)
			if err != nil {
				return AssetFacts{}, err
			}
			hasAlpha = chunk[0]&0x10 != 0
		case "VP8 ":
			frameWidth, frameHeight, err := parseVP8(chunk)
			if err != nil {
				return AssetFacts{}, err
			}
			if width == 0 {
				width, height = frameWidth, frameHeight
			}
			sawImage = true
		case "VP8L":
			frameWidth, frameHeight, alpha, err := parseVP8L(chunk)
			if err != nil {
				return AssetFacts{}, err
			}
			if width == 0 {
				width, height = frameWidth, frameHeight
			}
			hasAlpha, sawImage = hasAlpha || alpha, true
		case "ALPH":
			hasAlpha = true
		case "ANMF":
			if len(chunk) < 16 {
				return AssetFacts{}, errors.New("invalid WebP animation frame")
			}
			sawImage = true
		}
		step := 8 + length
		if length%2 != 0 {
			step++
		}
		if step > len(data)-offset {
			return AssetFacts{}, errors.New("truncated WebP chunk padding")
		}
		offset += step
	}
	if !sawImage || width < 1 || height < 1 {
		return AssetFacts{}, errors.New("WebP has no supported image frame")
	}
	if err := validateImageDimensions(width, height); err != nil {
		return AssetFacts{}, fmt.Errorf("WebP %w", err)
	}
	return AssetFacts{Width: width, Height: height, HasAlpha: hasAlpha}, nil
}

func parseVP8X(data []byte) (int, int, error) {
	if len(data) != 10 {
		return 0, 0, errors.New("invalid WebP extended header")
	}
	width := 1 + (int(data[4]) | int(data[5])<<8 | int(data[6])<<16)
	height := 1 + (int(data[7]) | int(data[8])<<8 | int(data[9])<<16)
	return width, height, nil
}

func parseVP8(data []byte) (int, int, error) {
	if len(data) < 10 || !bytes.Equal(data[3:6], []byte{0x9d, 0x01, 0x2a}) {
		return 0, 0, errors.New("invalid WebP lossy frame")
	}
	width := int(binary.LittleEndian.Uint16(data[6:8]) & 0x3fff)
	height := int(binary.LittleEndian.Uint16(data[8:10]) & 0x3fff)
	return width, height, nil
}

func parseVP8L(data []byte) (int, int, bool, error) {
	if len(data) < 5 || data[0] != 0x2f {
		return 0, 0, false, errors.New("invalid WebP lossless frame")
	}
	bits := uint32(data[1]) | uint32(data[2])<<8 | uint32(data[3])<<16 | uint32(data[4])<<24
	width := int(bits&0x3fff) + 1
	height := int((bits>>14)&0x3fff) + 1
	return width, height, bits&(1<<28) != 0, nil
}

func validateImageDimensions(width, height int) error {
	if width < 1 || height < 1 || int64(width)*int64(height) > maxInspectionPixels {
		return errors.New("dimensions exceed inspection limits")
	}
	return nil
}

func imageFacts(data []byte) (AssetFacts, error) {
	format := detectedFormat(data, "")
	return inspectByFormat("", data, format)
}
