package processing

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultTextureQuality = 75
	defaultZstdLevel      = 6
	maxZstdLevel          = 22
)

var texturePrimaries = map[string]byte{
	"bt709":       1,
	"bt601-ebu":   2,
	"bt601-smpte": 3,
	"bt2020":      4,
	"ciexyz":      5,
	"aces":        6,
	"acescc":      7,
}

func normalizeTextureEncoding(options TextureEncodingOptions, width, height int) (TextureEncodingOptions, error) {
	encoding := strings.ToLower(strings.TrimSpace(options.Encoding))
	mode := strings.ToLower(strings.TrimSpace(options.Mode))
	if encoding != "" && mode != "" && encoding != mode {
		return TextureEncodingOptions{}, fmt.Errorf("encoding and mode must match")
	}
	if encoding == "" {
		encoding = mode
	}
	if encoding == "" {
		encoding = TextureEncodingETC1S
	}
	if encoding != TextureEncodingETC1S && encoding != TextureEncodingUASTCZstd {
		return TextureEncodingOptions{}, fmt.Errorf("encoding must be %s or %s", TextureEncodingETC1S, TextureEncodingUASTCZstd)
	}
	if width < 1 || height < 1 {
		return TextureEncodingOptions{}, fmt.Errorf("texture dimensions must be positive")
	}
	expectedMips := generatedMipLevels(width, height)
	if options.MipLevels < 0 || (options.MipLevels != 0 && options.MipLevels != expectedMips) {
		return TextureEncodingOptions{}, fmt.Errorf("mipLevels must be the generated full chain of %d", expectedMips)
	}
	transfer := strings.ToLower(strings.TrimSpace(options.TransferFunction))
	if transfer == "" {
		transfer = TextureTransferSRGB
	}
	if transfer != TextureTransferLinear && transfer != TextureTransferSRGB {
		return TextureEncodingOptions{}, fmt.Errorf("unsupported transfer function %q for Basis Universal", options.TransferFunction)
	}
	primaries := strings.ToLower(strings.TrimSpace(options.ColorPrimaries))
	if primaries == "" {
		primaries = TexturePrimariesBT709
	}
	if _, ok := texturePrimaries[primaries]; !ok {
		return TextureEncodingOptions{}, fmt.Errorf("unsupported color primaries %q", options.ColorPrimaries)
	}
	quality := options.Quality
	if quality == 0 {
		quality = defaultTextureQuality
	}
	if quality < 1 || quality > 100 {
		return TextureEncodingOptions{}, fmt.Errorf("texture quality must be between 1 and 100")
	}
	if encoding == TextureEncodingETC1S && options.ZstdLevel != 0 {
		return TextureEncodingOptions{}, fmt.Errorf("zstdLevel is only valid for %s", TextureEncodingUASTCZstd)
	}
	zstdLevel := options.ZstdLevel
	if encoding == TextureEncodingUASTCZstd {
		if zstdLevel == 0 {
			zstdLevel = defaultZstdLevel
		}
		if zstdLevel < 1 || zstdLevel > maxZstdLevel {
			return TextureEncodingOptions{}, fmt.Errorf("zstdLevel must be between 1 and %d", maxZstdLevel)
		}
	}
	return TextureEncodingOptions{Encoding: encoding, MipLevels: expectedMips, TransferFunction: transfer,
		ColorPrimaries: primaries, Quality: quality, ZstdLevel: zstdLevel}, nil
}

func generatedMipLevels(width, height int) int {
	levels := 1
	for width > 1 || height > 1 {
		width = (width + 1) / 2
		height = (height + 1) / 2
		levels++
	}
	return levels
}

func encodeKTX2Image(ctx context.Context, source image.Image, options TextureEncodingOptions, encoderDirectory string) ([]byte, TextureEncodingOptions, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	width, height := source.Bounds().Dx(), source.Bounds().Dy()
	normalized, err := normalizeTextureEncoding(options, width, height)
	if err != nil {
		return nil, TextureEncodingOptions{}, err
	}
	pngInput, err := encodePNG(source, png.BestSpeed)
	if err != nil {
		return nil, TextureEncodingOptions{}, fmt.Errorf("encode PNG input: %w", err)
	}
	data, err := runBasisU(ctx, pngInput, normalized, width, height, encoderDirectory)
	if err != nil {
		return nil, TextureEncodingOptions{}, err
	}
	return data, normalized, nil
}

func runBasisU(ctx context.Context, pngInput []byte, options TextureEncodingOptions, width, height int, encoderDirectory string) ([]byte, error) {
	executable, err := findNativeEncoder("basisu", "ktx2", encoderDirectory)
	if err != nil {
		return nil, err
	}
	temporary, err := os.MkdirTemp("", "fabricum-basis-")
	if err != nil {
		return nil, fmt.Errorf("create Basis workspace: %w", err)
	}
	defer os.RemoveAll(temporary)
	inputPath := filepath.Join(temporary, "input.png")
	outputPath := filepath.Join(temporary, "output.ktx2")
	if err := os.WriteFile(inputPath, pngInput, 0o600); err != nil {
		return nil, fmt.Errorf("stage PNG for basisu: %w", err)
	}
	command := exec.CommandContext(ctx, executable, basisUArgs(options, inputPath, outputPath)...)
	command.Dir = temporary
	command.Env = nativeToolEnvironment(executable)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("ktx2 encoding canceled: %w", ctxErr)
		}
		return nil, nativeToolStartError("ktx2", "basisu", executable, err)
	}
	if err := command.Wait(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("ktx2 encoding canceled: %w", ctxErr)
		}
		return nil, nativeToolRunError("ktx2", "basisu", executable, err, stderr.String())
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read basisu output: %w", err)
	}
	if !validNativeOutput("ktx2", data) {
		return nil, fmt.Errorf("basisu encoder produced invalid KTX2 output")
	}
	if err := patchKTX2Metadata(data, options); err != nil {
		return nil, err
	}
	facts, err := inspectKTX2(data)
	if err != nil {
		return nil, fmt.Errorf("inspect basisu output: %w", err)
	}
	if facts.Width != width || facts.Height != height || facts.MipLevels != options.MipLevels || facts.Encoding != options.Encoding || facts.TransferFunction != options.TransferFunction || facts.ColorPrimaries != options.ColorPrimaries {
		return nil, fmt.Errorf("basisu output contract mismatch: got %dx%d, %d mips, %s, %s/%s", facts.Width, facts.Height, facts.MipLevels, facts.Encoding, facts.TransferFunction, facts.ColorPrimaries)
	}
	return data, nil
}

func basisUArgs(options TextureEncodingOptions, inputPath, outputPath string) []string {
	args := []string{"-quiet", "-ktx2", "-mipmap", "-no_multithreading", "-no_alpha", "-quality", strconv.Itoa(options.Quality)}
	if options.Encoding == TextureEncodingUASTCZstd {
		args = append(args, "-uastc", "-ktx2_zstandard_level", strconv.Itoa(options.ZstdLevel))
	} else {
		args = append(args, "-etc1s")
	}
	if options.TransferFunction == TextureTransferLinear {
		args = append(args, "-linear")
	} else {
		args = append(args, "-srgb")
	}
	return append(args, "-output_file", outputPath, "-file", inputPath)
}

func patchKTX2Metadata(data []byte, options TextureEncodingOptions) error {
	if !isKTX2(data) || len(data) < 80 {
		return fmt.Errorf("basisu encoder produced an invalid KTX2 header")
	}
	dfdOffset := binary.LittleEndian.Uint32(data[48:52])
	dfdLength := binary.LittleEndian.Uint32(data[52:56])
	if uint64(dfdOffset) > uint64(len(data)) || uint64(dfdLength) > uint64(len(data))-uint64(dfdOffset) {
		return fmt.Errorf("basisu KTX2 descriptor exceeds file bounds")
	}
	if dfdLength < 12 {
		return fmt.Errorf("basisu KTX2 descriptor is too short")
	}
	descriptor := data[int(dfdOffset):int(uint64(dfdOffset)+uint64(dfdLength))]
	dfdTotalSize := binary.LittleEndian.Uint32(descriptor[:4])
	if uint64(dfdTotalSize) != uint64(dfdLength) {
		return fmt.Errorf("basisu KTX2 descriptor has an invalid total size")
	}
	blockSize := int(binary.LittleEndian.Uint16(descriptor[10:12]))
	if blockSize < 24 || blockSize > len(descriptor)-4 {
		return fmt.Errorf("basisu KTX2 descriptor has an invalid block size")
	}
	transfer := map[string]byte{TextureTransferLinear: 1, TextureTransferSRGB: 2}[options.TransferFunction]
	primaries := texturePrimaries[options.ColorPrimaries]
	descriptor[13], descriptor[14] = primaries, transfer
	return nil
}
