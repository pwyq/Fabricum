package processing

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

var ktx2Magic = []byte{0xab, 'K', 'T', 'X', ' ', '2', '0', 0xbb, 0x0d, 0x0a, 0x1a, 0x0a}

func isKTX2(data []byte) bool {
	return len(data) >= len(ktx2Magic) && bytes.Equal(data[:len(ktx2Magic)], ktx2Magic)
}

func inspectKTX2(data []byte) (AssetFacts, error) {
	if !isKTX2(data) || len(data) < 80 {
		return AssetFacts{}, errors.New("invalid KTX2 header")
	}
	width, height := int(binary.LittleEndian.Uint32(data[20:24])), int(binary.LittleEndian.Uint32(data[24:28]))
	if height == 0 {
		height = 1
	}
	if err := validateImageDimensions(width, height); err != nil {
		return AssetFacts{}, fmt.Errorf("KTX2 %w", err)
	}
	levelCount := int(binary.LittleEndian.Uint32(data[40:44]))
	if levelCount == 0 {
		levelCount = 1
	}
	if levelCount > 32 || uint64(80)+uint64(levelCount)*24 > uint64(len(data)) {
		return AssetFacts{}, errors.New("invalid KTX2 level index")
	}
	for offset := 80; offset < 80+levelCount*24; offset += 24 {
		levelOffset := binary.LittleEndian.Uint64(data[offset : offset+8])
		levelLength := binary.LittleEndian.Uint64(data[offset+8 : offset+16])
		if levelOffset > uint64(len(data)) || levelLength > uint64(len(data))-levelOffset {
			return AssetFacts{}, errors.New("KTX2 level exceeds file bounds")
		}
	}
	dfdOffset := binary.LittleEndian.Uint32(data[48:52])
	dfdLength := binary.LittleEndian.Uint32(data[52:56])
	if uint64(dfdOffset) > uint64(len(data)) || uint64(dfdLength) > uint64(len(data))-uint64(dfdOffset) {
		return AssetFacts{}, errors.New("KTX2 data format descriptor exceeds file bounds")
	}
	encoding, transfer, primaries, err := parseKTX2Descriptor(data[dfdOffset:uint64(dfdOffset)+uint64(dfdLength)], binary.LittleEndian.Uint32(data[12:16]), binary.LittleEndian.Uint32(data[44:48]))
	if err != nil {
		return AssetFacts{}, err
	}
	return AssetFacts{Width: width, Height: height, MipLevels: levelCount, Encoding: encoding, TransferFunction: transfer, ColorPrimaries: primaries}, nil
}

func parseKTX2Descriptor(data []byte, vkFormat, supercompression uint32) (string, string, string, error) {
	if len(data) < 12 {
		return "", "", "", errors.New("KTX2 data format descriptor is too short")
	}
	dfdTotalSize := binary.LittleEndian.Uint32(data[:4])
	if uint64(dfdTotalSize) != uint64(len(data)) {
		return "", "", "", errors.New("invalid KTX2 data format descriptor total size")
	}
	blockSize := int(binary.LittleEndian.Uint16(data[10:12]))
	if blockSize < 24 || blockSize > len(data)-4 {
		return "", "", "", errors.New("invalid KTX2 data format descriptor size")
	}
	model, primaries, transfer := data[12], data[13], data[14]
	encoding, err := ktx2Encoding(model, vkFormat, supercompression)
	if err != nil {
		return "", "", "", err
	}
	return encoding, ktx2TransferFunction(transfer), ktx2ColorPrimaries(primaries), nil
}

func ktx2Encoding(model byte, vkFormat, supercompression uint32) (string, error) {
	switch supercompression {
	case 0:
	case 1:
		return "etc1s", nil
	case 2:
		if model == 166 {
			return "uastc-zstd", nil
		}
		return "zstd", nil
	case 3:
		return "zlib", nil
	default:
		return "", fmt.Errorf("unsupported KTX2 supercompression scheme %d", supercompression)
	}
	switch model {
	case 163:
		return "etc1s", nil
	case 166:
		return "uastc", nil
	case 1, 3:
		return "uncompressed", nil
	default:
		if vkFormat != 0 {
			return fmt.Sprintf("vk-format-%d", vkFormat), nil
		}
		return fmt.Sprintf("data-format-%d", model), nil
	}
}

func ktx2TransferFunction(value byte) string {
	switch value {
	case 1:
		return "linear"
	case 2:
		return "srgb"
	case 3:
		return "itu601"
	case 4:
		return "itu709"
	case 5:
		return "itu2020"
	default:
		return fmt.Sprintf("unknown-%d", value)
	}
}

func ktx2ColorPrimaries(value byte) string {
	switch value {
	case 1:
		return "bt709"
	case 2:
		return "bt601-ebu"
	case 3:
		return "bt601-smpte"
	case 4:
		return "bt2020"
	case 5:
		return "ciexyz"
	case 6:
		return "aces"
	case 7:
		return "acescc"
	default:
		return fmt.Sprintf("unknown-%d", value)
	}
}
