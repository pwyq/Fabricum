package processing

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestStaticModelPreprocessingUsesPinnedToolContract(t *testing.T) {
	root := t.TempDir()
	modelPath := filepath.Join(root, "prop.gltf")
	outputPath := filepath.Join(root, "delivery", "prop.gltf")
	writeSyntheticStaticModel(t, modelPath)
	toolDirectory := filepath.Join(root, "tool")
	if err := os.MkdirAll(toolDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFakeGltfpack(t, toolDirectory)
	t.Setenv("PATH", toolDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))

	result, err := PrepareModelOutputs(nil, ModelOptimizationRequest{
		Source: modelPath, Output: outputPath, Compression: ModelCompressionNone,
		BakeRootTransform: true, CenterXZAtGround: true, DeduplicateMaterials: true,
		CompactBuffers: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Outputs) != 1 {
		t.Fatalf("prepared outputs = %d, want one", len(result.Outputs))
	}
	measurement := result.Outputs[0].Measurement
	if measurement.ToolVersion != GltfpackVersion || measurement.TriangleCount != 1 || measurement.MaterialCount != 1 || len(measurement.SHA256) != 64 {
		t.Fatalf("unexpected model receipt: %+v", measurement)
	}
	if measurement.Bounds == nil || measurement.Bounds.Min != [3]float64{-0.5, 0, -1} || measurement.Bounds.Max != [3]float64{0.5, 2, 1} {
		t.Fatalf("unexpected centered bounds: %+v", measurement.Bounds)
	}
	object, err := decodeModelObject(result.Outputs[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	nodes, _ := modelArray(object, "nodes")
	if _, ok := nodes[0]["scale"]; ok {
		t.Fatal("root scale was not baked")
	}
	materials, _ := modelArray(object, "materials")
	if len(materials) != 1 {
		t.Fatalf("materials = %d, want one", len(materials))
	}
	meshes, _ := modelArray(object, "meshes")
	primitives, _ := modelArray(meshes[0], "primitives")
	var material int
	if err := json.Unmarshal(primitives[0]["material"], &material); err != nil || material != 0 {
		t.Fatalf("primitive material = %s, want 0", primitives[0]["material"])
	}
	var indices int
	if err := json.Unmarshal(primitives[0]["indices"], &indices); err != nil {
		t.Fatal(err)
	}
	accessors, _ := modelArray(object, "accessors")
	views, _ := modelArray(object, "bufferViews")
	if len(views) != 2 || len(accessors) != 2 {
		t.Fatalf("compacted geometry = %d views, %d accessors", len(views), len(accessors))
	}
	bufferURI, _ := modelString(mustModelArrayEntry(object, "buffers", 0), "uri")
	data, err := decodeDataURI(bufferURI)
	if err != nil {
		t.Fatal(err)
	}
	view := views[indices]
	byteOffset, _, _ := modelInt(view, "byteOffset")
	if len(data) < byteOffset+6 {
		t.Fatal("compacted index buffer is truncated")
	}
	if readModelIndex(data[byteOffset:byteOffset+2], 5123) != 0 || readModelIndex(data[byteOffset+2:byteOffset+4], 5123) != 2 || readModelIndex(data[byteOffset+4:byteOffset+6], 5123) != 1 {
		t.Fatal("mirrored triangle winding was not repaired")
	}
}

func TestMirroredWindingRepairsNonIndexedTriangles(t *testing.T) {
	root := t.TempDir()
	modelPath := filepath.Join(root, "non-indexed.gltf")
	writeSyntheticStaticModel(t, modelPath)
	document, err := loadModelDocument(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	meshes, err := modelArray(document.Object, "meshes")
	if err != nil {
		t.Fatal(err)
	}
	primitives, err := modelArray(meshes[0], "primitives")
	if err != nil {
		t.Fatal(err)
	}
	delete(primitives[0], "indices")
	if err := setModelArray(meshes[0], "primitives", primitives); err != nil {
		t.Fatal(err)
	}
	if err := setModelArray(document.Object, "meshes", meshes); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareModelGeometry(&document, ModelOptimizationRequest{BakeRootTransform: true}); err != nil {
		t.Fatal(err)
	}
	meshes, _ = modelArray(document.Object, "meshes")
	primitives, _ = modelArray(meshes[0], "primitives")
	var index int
	if err := json.Unmarshal(primitives[0]["indices"], &index); err != nil {
		t.Fatal(err)
	}
	accessors, _ := modelArray(document.Object, "accessors")
	views, _ := modelArray(document.Object, "bufferViews")
	viewIndex, _, _ := modelInt(accessors[index], "bufferView")
	viewOffset, _, _ := modelInt(views[viewIndex], "byteOffset")
	data := document.Buffers[0]
	if readModelIndex(data[viewOffset:viewOffset+4], 5125) != 0 || readModelIndex(data[viewOffset+4:viewOffset+8], 5125) != 2 || readModelIndex(data[viewOffset+8:viewOffset+12], 5125) != 1 {
		t.Fatal("mirrored non-indexed triangle winding was not repaired")
	}
}

func writeSyntheticStaticModel(t *testing.T, path string) {
	data := make([]byte, 36+6)
	for index, value := range []float32{1, 2, 3, 2, 2, 3, 1, 4, 5} {
		binary.LittleEndian.PutUint32(data[index*4:], math.Float32bits(value))
	}
	for index, value := range []uint16{0, 1, 2} {
		binary.LittleEndian.PutUint16(data[36+index*2:], value)
	}
	material := map[string]any{"name": "leaf", "pbrMetallicRoughness": map[string]any{"metallicFactor": 0}}
	document := map[string]any{
		"asset":       map[string]string{"version": "2.0"},
		"buffers":     []any{map[string]any{"byteLength": len(data), "uri": "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(data)}},
		"bufferViews": []any{map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": 36}, map[string]any{"buffer": 0, "byteOffset": 36, "byteLength": 6}, map[string]any{"buffer": 0, "byteOffset": 42, "byteLength": 2}},
		"accessors": []any{
			map[string]any{"bufferView": 0, "componentType": 5126, "count": 3, "type": "VEC3", "min": []float64{1, 2, 3}, "max": []float64{2, 4, 5}},
			map[string]any{"bufferView": 1, "componentType": 5123, "count": 3, "type": "SCALAR", "min": []int{0}, "max": []int{2}},
		},
		"materials": []any{material, material},
		"meshes":    []any{map[string]any{"primitives": []any{map[string]any{"attributes": map[string]int{"POSITION": 0}, "indices": 1, "material": 1}}}},
		"nodes":     []any{map[string]any{"mesh": 0, "scale": []float64{-1, 1, 1}}},
		"scenes":    []any{map[string]any{"nodes": []int{0}}}, "scene": 0,
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
}

func writeFakeGltfpack(t *testing.T, directory string) {
	if runtime.GOOS == "windows" {
		if err := os.WriteFile(filepath.Join(directory, "gltfpack.cmd"), []byte("@echo off\r\ncopy /Y \"%~2\" \"%~4\" >NUL\r\n"), 0700); err != nil {
			t.Fatal(err)
		}
		return
	}
	data := []byte("#!/bin/sh\nwhile [ \"$#\" -gt 0 ]; do\n  case \"$1\" in\n    -i) input=$2; shift 2;;\n    -o) output=$2; shift 2;;\n    *) shift;;\n  esac\ndone\ncp \"$input\" \"$output\"\n")
	if err := os.WriteFile(filepath.Join(directory, "gltfpack"), data, 0700); err != nil {
		t.Fatal(err)
	}
}

func mustModelArrayEntry(object map[string]json.RawMessage, key string, index int) map[string]json.RawMessage {
	values, err := modelArray(object, key)
	if err != nil || index < 0 || index >= len(values) {
		panic(err)
	}
	return values[index]
}
