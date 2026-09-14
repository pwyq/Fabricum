package processing

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	InspectionSchemaVersion = 1
	maxInspectionFiles      = 1024
	maxInspectionPathBytes  = 4096
	maxInspectionFileBytes  = 256 << 20
	maxInspectionPixels     = 64 << 20
	maxInspectionTextures   = 4096
)

// AssetFacts contains format-specific facts that are safe to consume from a
// project asset manifest. It intentionally has no content digest.
type AssetFacts struct {
	Path             string         `json:"path"`
	Format           string         `json:"format,omitempty"`
	Width            int            `json:"width,omitempty"`
	Height           int            `json:"height,omitempty"`
	HasAlpha         bool           `json:"hasAlpha"`
	Bytes            int64          `json:"bytes"`
	MipLevels        int            `json:"mipLevels,omitempty"`
	Encoding         string         `json:"encoding,omitempty"`
	TransferFunction string         `json:"transferFunction,omitempty"`
	ColorPrimaries   string         `json:"colorPrimaries,omitempty"`
	TriangleCount    int            `json:"triangleCount,omitempty"`
	PrimitiveCount   int            `json:"primitiveCount,omitempty"`
	MaterialCount    int            `json:"materialCount,omitempty"`
	TextureCount     int            `json:"textureCount,omitempty"`
	Textures         []TextureFacts `json:"textures,omitempty"`
	AnimationCount   int            `json:"animationCount,omitempty"`
	Bounds           *Bounds        `json:"bounds,omitempty"`
	Error            string         `json:"error,omitempty"`
}

// InspectionReport is the versioned, project-neutral output of an asset
// inspection invocation.
type InspectionReport struct {
	SchemaVersion int          `json:"schemaVersion"`
	Processor     string       `json:"processor"`
	Assets        []AssetFacts `json:"assets"`
}

// TextureFacts contains dimensions for an embedded texture that the
// inspector can identify without decoding the texture.
type TextureFacts struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Bounds contains the minimum and maximum XYZ coordinates of rendered data.
type Bounds struct {
	Min [3]float64 `json:"min"`
	Max [3]float64 `json:"max"`
}

// InspectFiles inspects files in order and returns one result for every path.
// File-level failures are represented in the result instead of aborting the
// batch; invalid batch arguments return an error before files are read.
func InspectFiles(paths []string) ([]AssetFacts, error) {
	if len(paths) == 0 {
		return nil, errors.New("at least one asset path is required")
	}
	if len(paths) > maxInspectionFiles {
		return nil, fmt.Errorf("at most %d asset paths may be inspected at once", maxInspectionFiles)
	}
	results := make([]AssetFacts, 0, len(paths))
	for _, path := range paths {
		result := AssetFacts{Path: path}
		if len(path) == 0 || len(path) > maxInspectionPathBytes {
			result.Error = "asset path must contain between 1 and 4096 bytes"
			results = append(results, result)
			continue
		}
		facts, err := inspectFile(path)
		if err != nil {
			result.Error = boundedInspectionError(err)
		} else {
			result = facts
			result.Path = path
		}
		results = append(results, result)
	}
	return results, nil
}

func HasInspectionErrors(results []AssetFacts) bool {
	for _, result := range results {
		if result.Error != "" {
			return true
		}
	}
	return false
}

func inspectFile(path string) (AssetFacts, error) {
	info, err := os.Stat(path)
	if err != nil {
		return AssetFacts{}, fmt.Errorf("read asset: %w", err)
	}
	if !info.Mode().IsRegular() {
		return AssetFacts{}, errors.New("asset is not a regular file")
	}
	if info.Size() > maxInspectionFileBytes {
		return AssetFacts{}, fmt.Errorf("asset exceeds %d byte inspection limit", maxInspectionFileBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return AssetFacts{}, fmt.Errorf("read asset: %w", err)
	}
	expected := expectedFormat(path)
	if expected == "" {
		return AssetFacts{}, fmt.Errorf("unsupported asset extension %q", filepath.Ext(path))
	}
	if expected == "bin" {
		return AssetFacts{Format: "bin", Bytes: int64(len(data))}, nil
	}
	actual := detectedFormat(data, expected)
	if actual != expected {
		return AssetFacts{}, fmt.Errorf("file content is %s, but path requires %s", actualFormatName(actual), expected)
	}
	facts, err := inspectByFormat(path, data, expected)
	if err != nil {
		return AssetFacts{}, err
	}
	facts.Format = expected
	facts.Bytes = int64(len(data))
	return facts, nil
}

func expectedFormat(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "png"
	case ".jpg", ".jpeg":
		return "jpeg"
	case ".gif":
		return "gif"
	case ".webp":
		return "webp"
	case ".avif":
		return "avif"
	case ".ktx2":
		return "ktx2"
	case ".gltf":
		return "gltf"
	case ".glb":
		return "glb"
	case ".bin":
		return "bin"
	default:
		return ""
	}
}

func detectedFormat(data []byte, expected string) string {
	switch {
	case isPNG(data):
		return "png"
	case isJPEG(data):
		return "jpeg"
	case isGIF(data):
		return "gif"
	case isWebP(data):
		return "webp"
	case isAVIF(data):
		return "avif"
	case isKTX2(data):
		return "ktx2"
	case isGLB(data):
		return "glb"
	case expected == "gltf" && isLikelyGLTFJSON(data):
		return "gltf"
	default:
		return "unknown"
	}
}

func actualFormatName(format string) string {
	if format == "unknown" {
		return "unknown data"
	}
	return format
}

func boundedInspectionError(err error) string {
	const maxErrorBytes = 512
	message := err.Error()
	if len(message) <= maxErrorBytes {
		return message
	}
	return message[:maxErrorBytes-3] + "..."
}

func inspectByFormat(path string, data []byte, format string) (AssetFacts, error) {
	switch format {
	case "png":
		return inspectPNG(data)
	case "jpeg":
		return inspectJPEG(data)
	case "gif":
		return inspectGIF(data)
	case "webp":
		return inspectWebP(data)
	case "avif":
		return inspectAVIF(data)
	case "ktx2":
		return inspectKTX2(data)
	case "gltf":
		object, err := decodeModelObject(data)
		if err != nil {
			return AssetFacts{}, err
		}
		binaryData, err := modelGLTFBuffer(path, object)
		if err != nil {
			return AssetFacts{}, err
		}
		return inspectGLTF(path, data, binaryData)
	case "glb":
		return inspectGLB(path, data)
	default:
		return AssetFacts{}, fmt.Errorf("unsupported asset format %q", format)
	}
}
