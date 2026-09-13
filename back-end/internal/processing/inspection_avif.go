package processing

import (
	"bytes"
	"encoding/binary"
	"errors"
)

func isAVIF(data []byte) bool {
	if len(data) < 16 || string(data[4:8]) != "ftyp" {
		return false
	}
	size := int64(binary.BigEndian.Uint32(data[:4]))
	if size < 16 || size > int64(len(data)) {
		return false
	}
	for offset := 8; offset+4 <= int(size); offset += 4 {
		if avifBrand(data[offset : offset+4]) {
			return true
		}
	}
	return false
}

func avifBrand(brand []byte) bool {
	return bytes.Equal(brand, []byte("avif")) || bytes.Equal(brand, []byte("avis"))
}

func inspectAVIF(data []byte) (AssetFacts, error) {
	if !isAVIF(data) {
		return AssetFacts{}, errors.New("invalid AVIF file type")
	}
	var width, height int
	var hasAlpha bool
	err := walkBMFFBoxes(data, func(name string, payload []byte) error {
		switch name {
		case "ispe":
			if len(payload) < 12 {
				return errors.New("truncated AVIF image spatial extents")
			}
			if width == 0 {
				width = int(binary.BigEndian.Uint32(payload[4:8]))
				height = int(binary.BigEndian.Uint32(payload[8:12]))
			}
		case "auxC", "infe":
			if bytes.Contains(bytes.ToLower(payload), []byte("alpha")) {
				hasAlpha = true
			}
		case "pixi":
			if len(payload) >= 5 && payload[4] >= 4 {
				hasAlpha = true
			}
		}
		return nil
	})
	if err != nil {
		return AssetFacts{}, err
	}
	if width == 0 || height == 0 {
		return AssetFacts{}, errors.New("AVIF has no image dimensions")
	}
	if err := validateImageDimensions(width, height); err != nil {
		return AssetFacts{}, err
	}
	return AssetFacts{Width: width, Height: height, HasAlpha: hasAlpha}, nil
}

func walkBMFFBoxes(data []byte, visit func(string, []byte) error) error {
	for offset := 0; offset < len(data); {
		if len(data)-offset < 8 {
			return errors.New("truncated ISO base media box")
		}
		size32 := binary.BigEndian.Uint32(data[offset : offset+4])
		headerSize := 8
		var size uint64
		switch size32 {
		case 0:
			size = uint64(len(data) - offset)
		case 1:
			if len(data)-offset < 16 {
				return errors.New("truncated extended media box size")
			}
			headerSize = 16
			size = binary.BigEndian.Uint64(data[offset+8 : offset+16])
		default:
			size = uint64(size32)
		}
		if size < uint64(headerSize) || size > uint64(len(data)-offset) {
			return errors.New("invalid ISO base media box size")
		}
		end := offset + int(size)
		name := string(data[offset+4 : offset+8])
		payload := data[offset+headerSize : end]
		if err := visit(name, payload); err != nil {
			return err
		}
		if bmffContainer(name) {
			children := payload
			if bmffFullBoxContainer(name) {
				if len(children) < 4 {
					return errors.New("truncated ISO base media container")
				}
				children = children[4:]
			}
			if len(children) > 0 {
				if err := walkBMFFBoxes(children, visit); err != nil {
					return err
				}
			}
		}
		offset = end
	}
	return nil
}

func bmffContainer(name string) bool {
	switch name {
	case "meta", "iprp", "ipco", "iref", "moov", "trak", "mdia", "minf", "stbl", "dinf", "edts", "mvex", "moof", "traf", "mfra", "udta", "sinf", "schi":
		return true
	default:
		return false
	}
}

func bmffFullBoxContainer(name string) bool {
	return name == "meta" || name == "iref"
}
