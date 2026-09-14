package processing

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxModelRequestBytes = 4096

func normalizeModelRequest(request ModelOptimizationRequest) (ModelOptimizationRequest, error) {
	if request.Source == "" || len(request.Source) > maxModelRequestBytes {
		return ModelOptimizationRequest{}, errors.New("model source path must contain between 1 and 4096 bytes")
	}
	if request.Output == "" || len(request.Output) > maxModelRequestBytes {
		return ModelOptimizationRequest{}, errors.New("model output path must contain between 1 and 4096 bytes")
	}
	var err error
	request.Source, err = filepath.Abs(request.Source)
	if err != nil {
		return ModelOptimizationRequest{}, fmt.Errorf("resolve model source: %w", err)
	}
	request.Output, err = filepath.Abs(request.Output)
	if err != nil {
		return ModelOptimizationRequest{}, fmt.Errorf("resolve model output: %w", err)
	}
	if sameTransformPath(request.Source, request.Output) {
		return ModelOptimizationRequest{}, errors.New("model source and output paths must be distinct")
	}
	if _, err := os.Stat(request.Source); err != nil {
		return ModelOptimizationRequest{}, fmt.Errorf("read model source: %w", err)
	}
	if ext := strings.ToLower(filepath.Ext(request.Source)); ext != ".gltf" && ext != ".glb" {
		return ModelOptimizationRequest{}, errors.New("model source must end in .gltf or .glb")
	}
	if ext := strings.ToLower(filepath.Ext(request.Output)); ext != ".gltf" && ext != ".glb" {
		return ModelOptimizationRequest{}, errors.New("model output must end in .gltf or .glb")
	}
	compression := strings.ToLower(strings.TrimSpace(request.Compression))
	meshCompression := strings.ToLower(strings.TrimSpace(request.MeshCompression))
	if compression != "" && meshCompression != "" && compression != meshCompression {
		return ModelOptimizationRequest{}, errors.New("compression and meshCompression must match")
	}
	if compression == "" {
		compression = meshCompression
	}
	if compression == "" {
		compression = ModelCompressionNone
	}
	if compression == "ext" || compression == "ext_meshopt" {
		compression = ModelCompressionMeshopt
	}
	if compression != ModelCompressionNone && compression != ModelCompressionMeshopt {
		return ModelOptimizationRequest{}, errors.New("compression must be none or meshopt")
	}
	request.Compression, request.MeshCompression = compression, compression
	texture := strings.ToLower(strings.TrimSpace(request.TextureCompression))
	if texture == "" {
		texture = ModelTextureNone
	}
	if texture == "basis" || texture == "basisu" {
		texture = ModelTextureKTX2
	}
	if texture != ModelTextureNone && texture != ModelTextureKTX2 && texture != ModelTextureWebP {
		return ModelOptimizationRequest{}, errors.New("textureCompression must be none, ktx2, or webp")
	}
	request.TextureCompression = texture
	encoding := strings.ToLower(strings.TrimSpace(request.TextureEncoding))
	if encoding == "" {
		encoding = ModelTextureETC1S
	}
	if encoding != ModelTextureETC1S && encoding != ModelTextureUASTC {
		return ModelOptimizationRequest{}, errors.New("textureEncoding must be etc1s or uastc")
	}
	if texture != ModelTextureKTX2 && request.TextureEncoding != "" {
		return ModelOptimizationRequest{}, errors.New("textureEncoding requires ktx2 texture compression")
	}
	request.TextureEncoding = encoding
	if request.TextureQuality == 0 {
		request.TextureQuality = 8
	}
	if request.TextureQuality < 1 || request.TextureQuality > 10 {
		return ModelOptimizationRequest{}, errors.New("textureQuality must be between 1 and 10")
	}
	request.GltfpackDirectory, err = absoluteOptionalModelPath(request.GltfpackDirectory)
	if err != nil {
		return ModelOptimizationRequest{}, err
	}
	request.EncoderDirectory, err = absoluteOptionalModelPath(request.EncoderDirectory)
	if err != nil {
		return ModelOptimizationRequest{}, err
	}
	return request, nil
}

func absoluteOptionalModelPath(path string) (string, error) {
	if path == "" || filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Abs(path)
}

func modelOutputFormat(path string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
}
