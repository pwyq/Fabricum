package processing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"sort"
)

func modelMeasurement(path, format string, data []byte, facts AssetFacts, extensions, warnings []string, role string, versions map[string]string) ModelOutputMeasurement {
	hash := sha256.Sum256(data)
	return ModelOutputMeasurement{
		Role: role, Path: filepath.ToSlash(path), Format: format, Tool: "gltfpack/meshoptimizer", ToolVersion: GltfpackVersion,
		Bytes: len(data), SHA256: hex.EncodeToString(hash[:]), Bounds: facts.Bounds, TriangleCount: facts.TriangleCount,
		PrimitiveCount: facts.PrimitiveCount, MaterialCount: facts.MaterialCount, TextureCount: facts.TextureCount,
		AnimationCount: facts.AnimationCount, UsedExtensions: extensions, Warnings: warnings, NativeToolVersions: versions,
	}
}

func modelOutputExtensions(object map[string]json.RawMessage) []string {
	values := make(map[string]struct{})
	for _, key := range []string{"extensionsUsed", "extensionsRequired"} {
		var extensions []string
		if raw, ok := object[key]; ok && json.Unmarshal(raw, &extensions) == nil {
			for _, extension := range extensions {
				values[extension] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(values))
	for extension := range values {
		result = append(result, extension)
	}
	sort.Strings(result)
	return result
}

func modelNativeVersions(request ModelOptimizationRequest) map[string]string {
	versions := map[string]string{"gltfpack": GltfpackVersion, "meshoptimizer": GltfpackVersion}
	if request.TextureCompression == ModelTextureKTX2 {
		versions["basisu"] = BasisUniversalVersion
	}
	if request.TextureCompression == ModelTextureWebP {
		versions["libwebp"] = LibWebPVersion
	}
	return versions
}
