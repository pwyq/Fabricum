package compatibility

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

type fixtures struct {
	Root        string
	Source      string
	Transparent string
	JPEG        string
	GIF         string
	Normal      string
	AO          string
	Roughness   string
	Model       string
	ModelBin    string
}

func createFixtures(root string) (fixtures, error) {
	paths := fixtures{
		Root: root, Source: filepath.Join(root, "source.png"), Transparent: filepath.Join(root, "transparent.png"),
		JPEG: filepath.Join(root, "sample.jpg"), GIF: filepath.Join(root, "sample.gif"), Normal: filepath.Join(root, "normal.png"),
		AO: filepath.Join(root, "ao.png"), Roughness: filepath.Join(root, "roughness.png"), Model: filepath.Join(root, "prop.gltf"),
		ModelBin: filepath.Join(root, "prop.bin"),
	}
	if err := writeImage(paths.Source, patternedImage(1024, 1024, func(x, y int) color.NRGBA {
		return color.NRGBA{R: uint8((x*3 + y) % 256), G: uint8((y*3 + x) % 256), B: uint8((x + y*2) % 256), A: 255}
	}), "png"); err != nil {
		return fixtures{}, err
	}
	if err := writeImage(paths.Transparent, patternedImage(160, 80, func(x, y int) color.NRGBA {
		if x < 20 || x >= 140 || y < 16 || y >= 64 {
			return color.NRGBA{}
		}
		return color.NRGBA{R: uint8(80 + x%100), G: uint8(40 + y), B: 180, A: 255}
	}), "png"); err != nil {
		return fixtures{}, err
	}
	small := patternedImage(8, 6, func(x, y int) color.NRGBA {
		return color.NRGBA{R: uint8(x * 24), G: uint8(y * 32), B: uint8((x + y) * 16), A: 255}
	})
	if err := writeImage(paths.JPEG, small, "jpeg"); err != nil {
		return fixtures{}, err
	}
	if err := writeImage(paths.GIF, small, "gif"); err != nil {
		return fixtures{}, err
	}
	for path, pixel := range map[string]func(int, int) color.NRGBA{
		paths.Normal: func(x, y int) color.NRGBA {
			return color.NRGBA{R: uint8(120 + x%16), G: uint8(128 + y%16), B: 255, A: 255}
		},
		paths.AO:        func(x, y int) color.NRGBA { return color.NRGBA{R: uint8(64 + (x+y)%128), A: 255} },
		paths.Roughness: func(x, y int) color.NRGBA { return color.NRGBA{R: uint8(192 + (x+y)%64), A: 255} },
	} {
		if err := writeImage(path, patternedImage(1024, 1024, pixel), "png"); err != nil {
			return fixtures{}, err
		}
	}
	if err := writeStaticModel(paths); err != nil {
		return fixtures{}, err
	}
	return paths, nil
}

func patternedImage(width, height int, pixel func(int, int) color.NRGBA) image.Image {
	result := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			result.SetNRGBA(x, y, pixel(x, y))
		}
	}
	return result
}

func writeImage(path string, source image.Image, format string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create fixture %s: %w", filepath.Base(path), err)
	}
	var encodeErr error
	switch format {
	case "png":
		encodeErr = png.Encode(file, source)
	case "jpeg":
		encodeErr = jpeg.Encode(file, source, &jpeg.Options{Quality: 90})
	case "gif":
		encodeErr = gif.Encode(file, source, nil)
	default:
		encodeErr = fmt.Errorf("unsupported fixture image format %s", format)
	}
	if closeErr := file.Close(); encodeErr == nil {
		encodeErr = closeErr
	}
	if encodeErr != nil {
		return fmt.Errorf("write fixture %s: %w", filepath.Base(path), encodeErr)
	}
	return nil
}

func pngBytes(source image.Image) ([]byte, error) {
	var data bytes.Buffer
	if err := png.Encode(&data, source); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}

func writeStaticModel(paths fixtures) error {
	data := make([]byte, 60)
	for index, value := range []float32{1, 2, 3, 2, 4, 5, 1, 3, 5} {
		binary.LittleEndian.PutUint32(data[index*4:], math.Float32bits(value))
	}
	for index, value := range []uint16{0, 1, 2} {
		binary.LittleEndian.PutUint16(data[36+index*2:], value)
	}
	for index, value := range []byte{255, 0, 0, 255, 0, 255, 0, 255, 0, 0, 255, 255} {
		data[44+index] = value
	}
	if err := os.WriteFile(paths.ModelBin, data, 0600); err != nil {
		return fmt.Errorf("write model BIN: %w", err)
	}
	texture, err := pngBytes(patternedImage(4, 4, func(x, y int) color.NRGBA {
		return color.NRGBA{R: uint8(40 + x*20), G: uint8(60 + y*20), B: 180, A: 255}
	}))
	if err != nil {
		return fmt.Errorf("encode model texture: %w", err)
	}
	material := map[string]any{
		"name":                 "synthetic-material",
		"pbrMetallicRoughness": map[string]any{"metallicFactor": 0, "baseColorTexture": map[string]any{"index": 0}},
	}
	document := map[string]any{
		"asset":   map[string]string{"version": "2.0"},
		"buffers": []any{map[string]any{"byteLength": len(data), "uri": filepath.Base(paths.ModelBin)}},
		"bufferViews": []any{
			map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": 36, "target": 34962},
			map[string]any{"buffer": 0, "byteOffset": 36, "byteLength": 6, "target": 34963},
			map[string]any{"buffer": 0, "byteOffset": 44, "byteLength": 12, "target": 34962},
			map[string]any{"buffer": 0, "byteOffset": 56, "byteLength": 4},
		},
		"accessors": []any{
			map[string]any{"bufferView": 0, "componentType": 5126, "count": 3, "type": "VEC3", "min": []float64{1, 2, 3}, "max": []float64{2, 4, 5}},
			map[string]any{"bufferView": 1, "componentType": 5123, "count": 3, "type": "SCALAR", "min": []int{0}, "max": []int{2}},
			map[string]any{"bufferView": 2, "componentType": 5121, "count": 3, "type": "VEC4", "normalized": true},
		},
		"images":    []any{map[string]any{"uri": "data:image/png;base64," + base64.StdEncoding.EncodeToString(texture), "mimeType": "image/png"}},
		"textures":  []any{map[string]any{"source": 0}},
		"materials": []any{material, material},
		"meshes":    []any{map[string]any{"primitives": []any{map[string]any{"attributes": map[string]int{"POSITION": 0, "COLOR_0": 2}, "indices": 1, "material": 1}}}},
		"nodes":     []any{map[string]any{"mesh": 0, "scale": []int{-1, 1, 1}}},
		"scenes":    []any{map[string]any{"nodes": []int{0}}}, "scene": 0,
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("encode model fixture: %w", err)
	}
	if err := os.WriteFile(paths.Model, encoded, 0600); err != nil {
		return fmt.Errorf("write model fixture: %w", err)
	}
	return nil
}
