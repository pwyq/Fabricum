package processing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const maxModelRequestBytes = 4096

// OptimizeModel prepares, optimizes, validates, and atomically writes a
// static model and any loose glTF sidecars emitted by gltfpack.
func OptimizeModel(ctx context.Context, request ModelOptimizationRequest) ([]ModelOutputMeasurement, error) {
	result, err := PrepareModelOutputs(ctx, request)
	if err != nil {
		return nil, err
	}
	for _, output := range result.Outputs {
		if err := WriteFileAtomically(output.Path, output.Data); err != nil {
			return nil, fmt.Errorf("write model output %s: %w", output.Measurement.Path, err)
		}
	}
	measurements := make([]ModelOutputMeasurement, len(result.Outputs))
	for index, output := range result.Outputs {
		measurements[index] = output.Measurement
	}
	return measurements, nil
}

// PrepareModelOutputs returns model bytes and measurements without replacing
// any delivery files.
func PrepareModelOutputs(ctx context.Context, request ModelOptimizationRequest) (ModelOptimizationResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	normalized, err := normalizeModelRequest(request)
	if err != nil {
		return ModelOptimizationResult{}, err
	}
	document, err := loadModelDocument(normalized.Source)
	if err != nil {
		return ModelOptimizationResult{}, err
	}
	warnings, err := prepareModelGeometry(&document, normalized)
	if err != nil {
		return ModelOptimizationResult{}, err
	}
	warnings = append(warnings, unsupportedModelWarnings(document.Object)...)
	workspace, err := os.MkdirTemp("", "fabricum-gltfpack-")
	if err != nil {
		return ModelOptimizationResult{}, fmt.Errorf("create model workspace: %w", err)
	}
	defer os.RemoveAll(workspace)
	if err := stageModelImages(&document, workspace); err != nil {
		return ModelOptimizationResult{}, err
	}
	inputPath := filepath.Join(workspace, "input.gltf")
	if err := writeModelJSON(inputPath, document.Object); err != nil {
		return ModelOptimizationResult{}, err
	}
	outputFormat := modelOutputFormat(normalized.Output)
	resultPath := filepath.Join(workspace, "result."+outputFormat)
	executable, err := findGltfpack(normalized)
	if err != nil {
		return ModelOptimizationResult{}, err
	}
	stderr, err := runGltfpack(ctx, executable, normalized, inputPath, resultPath, workspace)
	if err != nil {
		return ModelOptimizationResult{}, err
	}
	warnings = append(warnings, toolWarnings(stderr)...)
	outputs, err := readModelOutputs(resultPath, normalized.Output, normalized, warnings)
	if err != nil {
		return ModelOptimizationResult{}, err
	}
	return ModelOptimizationResult{Outputs: outputs, Warnings: warnings}, nil
}

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

func findGltfpack(request ModelOptimizationRequest) (string, error) {
	directory := request.GltfpackDirectory
	if directory == "" {
		directory = request.EncoderDirectory
	}
	return findNativeTool("gltfpack", "model", directory)
}

func runGltfpack(ctx context.Context, executable string, request ModelOptimizationRequest, inputPath, outputPath, directory string) ([]byte, error) {
	command := exec.CommandContext(ctx, executable, modelGltfpackArgs(request, inputPath, outputPath)...)
	command.Dir = directory
	command.Env = nativeToolEnvironment(executable)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("gltfpack optimization canceled: %w", ctxErr)
		}
		return nil, nativeToolStartError("model", "gltfpack", executable, err)
	}
	if err := command.Wait(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("gltfpack optimization canceled: %w", ctxErr)
		}
		return nil, nativeToolRunError("model", "gltfpack", executable, err, stderr.String())
	}
	return stderr.Bytes(), nil
}

func modelGltfpackArgs(request ModelOptimizationRequest, inputPath, outputPath string) []string {
	args := []string{"-i", inputPath, "-o", outputPath, "-km"}
	if request.Compression == ModelCompressionMeshopt {
		args = append(args, "-c")
	} else {
		args = append(args, "-noq")
	}
	switch request.TextureCompression {
	case ModelTextureKTX2:
		args = append(args, "-tc")
		if request.TextureEncoding == ModelTextureUASTC {
			args = append(args, "-tu")
		}
		args = append(args, "-tq", fmt.Sprintf("%d", request.TextureQuality))
	case ModelTextureWebP:
		args = append(args, "-tw", "-tq", fmt.Sprintf("%d", request.TextureQuality))
	}
	return args
}

func writeModelJSON(path string, object map[string]json.RawMessage) error {
	data, err := json.Marshal(object)
	if err != nil {
		return fmt.Errorf("encode staged glTF: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		return fmt.Errorf("write staged glTF: %w", err)
	}
	return nil
}

func toolWarnings(data []byte) []string {
	var warnings []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "warning") {
			warnings = append(warnings, line)
		}
	}
	return warnings
}

func unsupportedModelWarnings(object map[string]json.RawMessage) []string {
	known := map[string]struct{}{
		"KHR_lights_punctual": {}, "KHR_materials_anisotropy": {}, "KHR_materials_clearcoat": {},
		"KHR_materials_diffuse_transmission": {}, "KHR_materials_dispersion": {}, "KHR_materials_emissive_strength": {},
		"KHR_materials_ior": {}, "KHR_materials_iridescence": {}, "KHR_materials_pbrSpecularGlossiness": {},
		"KHR_materials_sheen": {}, "KHR_materials_specular": {}, "KHR_materials_transmission": {},
		"KHR_materials_unlit": {}, "KHR_materials_variants": {}, "KHR_materials_volume": {},
		"KHR_mesh_quantization": {}, "KHR_meshopt_compression": {}, "KHR_texture_basisu": {},
		"KHR_texture_transform": {}, "EXT_mesh_gpu_instancing": {}, "EXT_meshopt_compression": {},
		"EXT_texture_webp": {},
	}
	var extensions []string
	for _, key := range []string{"extensionsUsed", "extensionsRequired"} {
		raw, ok := object[key]
		if !ok {
			continue
		}
		var values []string
		if json.Unmarshal(raw, &values) == nil {
			extensions = append(extensions, values...)
		}
	}
	var warnings []string
	seen := make(map[string]struct{})
	for _, extension := range extensions {
		if _, ok := known[extension]; !ok {
			if _, duplicate := seen[extension]; duplicate {
				continue
			}
			seen[extension] = struct{}{}
			warnings = append(warnings, fmt.Sprintf("unsupported extension %s may be discarded by gltfpack", extension))
		}
	}
	return warnings
}
