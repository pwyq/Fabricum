package processing

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

type glTFDocument struct {
	Asset struct {
		Version string `json:"version"`
	} `json:"asset"`
	Meshes     []gltfMesh        `json:"meshes"`
	Accessors  []gltfAccessor    `json:"accessors"`
	Materials  []json.RawMessage `json:"materials"`
	Textures   []json.RawMessage `json:"textures"`
	Images     []gltfImage       `json:"images"`
	BufferView []gltfBufferView  `json:"bufferViews"`
	Animations []json.RawMessage `json:"animations"`
}

type gltfMesh struct {
	Primitives []gltfPrimitive `json:"primitives"`
}

type gltfPrimitive struct {
	Attributes map[string]int `json:"attributes"`
	Indices    *int           `json:"indices"`
	Mode       *int           `json:"mode"`
}

type gltfAccessor struct {
	Count int       `json:"count"`
	Type  string    `json:"type"`
	Min   []float64 `json:"min"`
	Max   []float64 `json:"max"`
}

type gltfImage struct {
	URI        string `json:"uri"`
	MimeType   string `json:"mimeType"`
	BufferView *int   `json:"bufferView"`
}

type gltfBufferView struct {
	Buffer     int `json:"buffer"`
	ByteOffset int `json:"byteOffset"`
	ByteLength int `json:"byteLength"`
}

func isGLB(data []byte) bool { return len(data) >= 4 && bytes.Equal(data[:4], []byte("glTF")) }

func isLikelyGLTFJSON(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

func inspectGLTF(_ string, data, binaryData []byte) (AssetFacts, error) {
	var document glTFDocument
	if err := json.Unmarshal(bytes.TrimSpace(data), &document); err != nil {
		return AssetFacts{}, fmt.Errorf("decode glTF JSON: %w", err)
	}
	if document.Asset.Version != "2.0" {
		return AssetFacts{}, errors.New("glTF asset version must be 2.0")
	}
	if len(document.Images) > maxInspectionTextures {
		return AssetFacts{}, fmt.Errorf("glTF contains more than %d images", maxInspectionTextures)
	}
	primitiveCount, triangleCount, bounds, err := gltfMeshFacts(document)
	if err != nil {
		return AssetFacts{}, err
	}
	textures, err := embeddedGLTFTextures(document, binaryData)
	if err != nil {
		return AssetFacts{}, err
	}
	facts := AssetFacts{
		TriangleCount:  triangleCount,
		PrimitiveCount: primitiveCount,
		MaterialCount:  len(document.Materials),
		TextureCount:   len(document.Textures),
		Textures:       textures,
		AnimationCount: len(document.Animations),
		Bounds:         bounds,
	}
	return facts, nil
}

func inspectGLB(path string, data []byte) (AssetFacts, error) {
	if !isGLB(data) || len(data) < 12 || binary.LittleEndian.Uint32(data[4:8]) != 2 {
		return AssetFacts{}, errors.New("invalid GLB header")
	}
	if uint64(binary.LittleEndian.Uint32(data[8:12])) != uint64(len(data)) {
		return AssetFacts{}, errors.New("GLB length does not match file")
	}
	var jsonData, binaryData []byte
	for offset := 12; offset < len(data); {
		if len(data)-offset < 8 {
			return AssetFacts{}, errors.New("truncated GLB chunk")
		}
		length := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		if length%4 != 0 || length > len(data)-offset-8 {
			return AssetFacts{}, errors.New("invalid GLB chunk length")
		}
		chunkType := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		chunk := data[offset+8 : offset+8+length]
		switch chunkType {
		case 0x4e4f534a:
			if jsonData != nil || offset != 12 {
				return AssetFacts{}, errors.New("GLB must contain one JSON chunk first")
			}
			jsonData = bytes.TrimRight(chunk, "\x00 \t\r\n")
		case 0x004e4942:
			if binaryData != nil {
				return AssetFacts{}, errors.New("GLB contains multiple binary chunks")
			}
			binaryData = chunk
		}
		offset += 8 + length
	}
	if jsonData == nil {
		return AssetFacts{}, errors.New("GLB is missing its JSON chunk")
	}
	return inspectGLTF(path, jsonData, binaryData)
}

func gltfMeshFacts(document glTFDocument) (int, int, *Bounds, error) {
	primitiveCount, triangleCount := 0, 0
	var bounds *Bounds
	for _, mesh := range document.Meshes {
		for _, primitive := range mesh.Primitives {
			primitiveCount++
			mode := 4
			if primitive.Mode != nil {
				mode = *primitive.Mode
			}
			if mode < 0 || mode > 6 {
				return 0, 0, nil, fmt.Errorf("unsupported glTF primitive mode %d", mode)
			}
			count, err := gltfPrimitiveCount(document, primitive)
			if err != nil {
				return 0, 0, nil, err
			}
			triangleCount += trianglesForPrimitive(mode, count)
			position, ok := primitive.Attributes["POSITION"]
			if ok {
				accessorBounds, err := gltfAccessorBounds(document, position)
				if err != nil {
					return 0, 0, nil, err
				}
				bounds = mergeBounds(bounds, accessorBounds)
			}
		}
	}
	return primitiveCount, triangleCount, bounds, nil
}

func gltfPrimitiveCount(document glTFDocument, primitive gltfPrimitive) (int, error) {
	if primitive.Indices != nil {
		return gltfAccessorCount(document, *primitive.Indices)
	}
	position, ok := primitive.Attributes["POSITION"]
	if !ok {
		return 0, nil
	}
	return gltfAccessorCount(document, position)
}

func gltfAccessorCount(document glTFDocument, index int) (int, error) {
	if index < 0 || index >= len(document.Accessors) {
		return 0, fmt.Errorf("glTF accessor index %d is out of range", index)
	}
	if document.Accessors[index].Count < 0 {
		return 0, errors.New("glTF accessor count cannot be negative")
	}
	return document.Accessors[index].Count, nil
}

func trianglesForPrimitive(mode, count int) int {
	switch mode {
	case 4:
		return count / 3
	case 5, 6:
		if count < 3 {
			return 0
		}
		return count - 2
	default:
		return 0
	}
}

func gltfAccessorBounds(document glTFDocument, index int) (*Bounds, error) {
	if index < 0 || index >= len(document.Accessors) {
		return nil, fmt.Errorf("glTF POSITION accessor index %d is out of range", index)
	}
	accessor := document.Accessors[index]
	if len(accessor.Min) == 0 && len(accessor.Max) == 0 {
		return nil, nil
	}
	if len(accessor.Min) < 3 || len(accessor.Max) < 3 || accessor.Type != "VEC3" {
		return nil, errors.New("glTF POSITION accessor has invalid bounds")
	}
	result := &Bounds{}
	for axis := range 3 {
		result.Min[axis], result.Max[axis] = accessor.Min[axis], accessor.Max[axis]
		if math.IsNaN(result.Min[axis]) || math.IsNaN(result.Max[axis]) || math.IsInf(result.Min[axis], 0) || math.IsInf(result.Max[axis], 0) || result.Min[axis] > result.Max[axis] {
			return nil, errors.New("glTF POSITION accessor has invalid bounds")
		}
	}
	return result, nil
}

func mergeBounds(left, right *Bounds) *Bounds {
	if right == nil {
		return left
	}
	if left == nil {
		copy := *right
		return &copy
	}
	for axis := range 3 {
		left.Min[axis] = minFloat(left.Min[axis], right.Min[axis])
		left.Max[axis] = maxFloat(left.Max[axis], right.Max[axis])
	}
	return left
}

func minFloat(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}
