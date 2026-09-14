package processing

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectFilesReportsSyntheticAssetFactsInOrder(t *testing.T) {
	root := t.TempDir()
	pngPath := filepath.Join(root, "alpha.png")
	jpegPath := filepath.Join(root, "photo.jpg")
	gifPath := filepath.Join(root, "animation.gif")
	webpPath := filepath.Join(root, "photo.webp")
	avifPath := filepath.Join(root, "photo.avif")
	ktxPath := filepath.Join(root, "normal.ktx2")
	glbPath := filepath.Join(root, "model.glb")
	binPath := filepath.Join(root, "model.bin")
	writeInspectionPNG(t, pngPath, true)
	writeInspectionJPEG(t, jpegPath)
	writeInspectionGIF(t, gifPath)
	writeInspectionFile(t, webpPath, syntheticWebP(4, 3))
	writeInspectionFile(t, avifPath, syntheticAVIF(7, 5, true))
	writeInspectionFile(t, ktxPath, syntheticKTX2(16, 8, 3))
	pngData, err := os.ReadFile(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	writeInspectionFile(t, glbPath, syntheticGLB(t, pngData))
	writeInspectionFile(t, binPath, []byte{1, 2, 3, 4})

	paths := []string{pngPath, jpegPath, gifPath, webpPath, avifPath, ktxPath, glbPath, binPath}
	before, err := os.ReadFile(glbPath)
	if err != nil {
		t.Fatal(err)
	}
	results, err := InspectFiles(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(paths) {
		t.Fatalf("got %d results, want %d", len(results), len(paths))
	}
	for index, result := range results {
		if result.Path != paths[index] || result.Error != "" {
			t.Fatalf("result %d = %+v", index, result)
		}
	}
	if results[0].Format != "png" || results[0].Width != 4 || results[0].Height != 3 || !results[0].HasAlpha || results[0].Bytes < 1 {
		t.Fatalf("unexpected PNG facts: %+v", results[0])
	}
	if results[1].Format != "jpeg" || results[1].Width != 4 || results[1].Height != 3 || results[1].HasAlpha {
		t.Fatalf("unexpected JPEG facts: %+v", results[1])
	}
	if results[2].Format != "gif" || results[2].Width != 4 || results[2].Height != 3 {
		t.Fatalf("unexpected GIF facts: %+v", results[2])
	}
	if results[3].Format != "webp" || results[3].Width != 4 || results[3].Height != 3 || results[3].HasAlpha {
		t.Fatalf("unexpected WebP facts: %+v", results[3])
	}
	if results[4].Format != "avif" || results[4].Width != 7 || results[4].Height != 5 || !results[4].HasAlpha {
		t.Fatalf("unexpected AVIF facts: %+v", results[4])
	}
	if results[5].Format != "ktx2" || results[5].Width != 16 || results[5].Height != 8 || results[5].MipLevels != 3 || results[5].Encoding != "uastc-zstd" || results[5].TransferFunction != "srgb" || results[5].ColorPrimaries != "bt709" {
		t.Fatalf("unexpected KTX2 facts: %+v", results[5])
	}
	if results[6].Format != "glb" || results[6].TriangleCount != 2 || results[6].PrimitiveCount != 1 || results[6].MaterialCount != 1 || results[6].TextureCount != 1 || results[6].AnimationCount != 1 || results[6].Bounds == nil || results[6].Textures[0] != (TextureFacts{Width: 4, Height: 3}) {
		t.Fatalf("unexpected GLB facts: %+v", results[6])
	}
	if results[7].Format != "bin" || results[7].Bytes != 4 {
		t.Fatalf("unexpected BIN facts: %+v", results[7])
	}
	after, err := os.ReadFile(glbPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("inspection mutated an input")
	}
	report, err := json.Marshal(InspectionReport{InspectionSchemaVersion, "fabricum/test", results})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(report)), "sha256") || strings.Contains(strings.ToLower(string(report)), "hash") {
		t.Fatalf("inspection report exposes a digest: %s", report)
	}
}

func TestInspectFilesReturnsBoundedPerFileFailures(t *testing.T) {
	root := t.TempDir()
	malformed := filepath.Join(root, "bad.png")
	mismatch := filepath.Join(root, "wrong.jpg")
	unsupported := filepath.Join(root, "unknown.tga")
	writeInspectionFile(t, malformed, pngMagic)
	writeInspectionFile(t, mismatch, syntheticWebP(2, 2))
	writeInspectionFile(t, unsupported, []byte("not an asset"))
	results, err := InspectFiles([]string{filepath.Join(root, "missing.png"), malformed, mismatch, unsupported})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 || !HasInspectionErrors(results) {
		t.Fatalf("expected four failed results, got %+v", results)
	}
	for index, result := range results {
		if result.Error == "" || len(result.Error) > 512 {
			t.Fatalf("result %d has unbounded/missing error: %+v", index, result)
		}
	}
	if _, err := InspectFiles(nil); err == nil {
		t.Fatal("empty inspection batch should fail")
	}
	tooMany := make([]string, maxInspectionFiles+1)
	if _, err := InspectFiles(tooMany); err == nil {
		t.Fatal("oversized inspection batch should fail")
	}
}

func TestInspectLooseGLTFReadsExternalBinaryBufferForImages(t *testing.T) {
	root := t.TempDir()
	imagePath := filepath.Join(root, "texture.png")
	writeInspectionPNG(t, imagePath, false)
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	bufferPath := filepath.Join(root, "model.bin")
	writeInspectionFile(t, bufferPath, imageData)
	document := map[string]any{
		"asset":       map[string]string{"version": "2.0"},
		"buffers":     []any{map[string]any{"uri": "model.bin", "byteLength": len(imageData)}},
		"bufferViews": []any{map[string]any{"buffer": 0, "byteLength": len(imageData)}},
		"images":      []any{map[string]any{"bufferView": 0, "mimeType": "image/png"}},
		"textures":    []any{map[string]any{"source": 0}},
	}
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	gltfPath := filepath.Join(root, "model.gltf")
	writeInspectionFile(t, gltfPath, data)
	results, err := InspectFiles([]string{gltfPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Error != "" || results[0].Format != "gltf" || len(results[0].Textures) != 1 || results[0].Textures[0] != (TextureFacts{Width: 4, Height: 3}) {
		t.Fatalf("unexpected loose glTF facts: %+v", results)
	}
}

func TestInspectGLTFAppliesSceneTransformsToBounds(t *testing.T) {
	document := glTFDocument{
		Meshes:    []gltfMesh{{Primitives: []gltfPrimitive{{Attributes: map[string]int{"POSITION": 0}}}}},
		Accessors: []gltfAccessor{{Count: 3, Type: "VEC3", Min: []float64{1, 2, 3}, Max: []float64{2, 4, 5}}},
		Nodes:     []map[string]json.RawMessage{{"mesh": json.RawMessage("0"), "scale": json.RawMessage("[-1, 1, 1]")}},
		Scenes:    []map[string]json.RawMessage{{"nodes": json.RawMessage("[0]")}},
		Scene:     func() *int { value := 0; return &value }(),
	}
	_, _, bounds, err := gltfMeshFacts(document)
	if err != nil {
		t.Fatal(err)
	}
	if bounds == nil || bounds.Min != [3]float64{-2, 2, 3} || bounds.Max != [3]float64{-1, 4, 5} {
		t.Fatalf("transformed bounds = %+v", bounds)
	}
}

func TestInspectWebPExtendedHeaderReportsCanvasDimensions(t *testing.T) {
	width, height := 0x1234, 0x2345
	canvas := []byte{0, 0, 0, 0, byte(width - 1), byte((width - 1) >> 8), byte((width - 1) >> 16), byte(height - 1), byte((height - 1) >> 8), byte((height - 1) >> 16)}
	frame := []byte{0, 0, 0, 0x9d, 0x01, 0x2a, byte(width), byte(width >> 8), byte(height), byte(height >> 8)}
	data := make([]byte, 12+8+len(canvas)+8+len(frame))
	copy(data, []byte("RIFF"))
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))
	copy(data[8:12], []byte("WEBP"))
	copy(data[12:16], []byte("VP8X"))
	binary.LittleEndian.PutUint32(data[16:20], uint32(len(canvas)))
	copy(data[20:], canvas)
	offset := 20 + len(canvas)
	copy(data[offset:offset+4], []byte("VP8 "))
	binary.LittleEndian.PutUint32(data[offset+4:offset+8], uint32(len(frame)))
	copy(data[offset+8:], frame)
	facts, err := inspectWebP(data)
	if err != nil {
		t.Fatal(err)
	}
	if facts.Width != width || facts.Height != height {
		t.Fatalf("got %dx%d, want %dx%d", facts.Width, facts.Height, width, height)
	}
}

func writeInspectionPNG(t *testing.T, path string, alpha bool) {
	t.Helper()
	fixture := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 4; x++ {
			a := uint8(255)
			if alpha && x == 0 && y == 0 {
				a = 0
			}
			fixture.SetNRGBA(x, y, color.NRGBA{R: 20, G: 40, B: 60, A: a})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, fixture); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeInspectionJPEG(t *testing.T, path string) {
	t.Helper()
	fixture := image.NewRGBA(image.Rect(0, 0, 4, 3))
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(file, fixture, nil); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeInspectionGIF(t *testing.T, path string) {
	t.Helper()
	palette := color.Palette{color.Transparent, color.White}
	frame := image.NewPaletted(image.Rect(0, 0, 4, 3), palette)
	for index := range frame.Pix {
		frame.Pix[index] = 1
	}
	frame.Pix[0] = 0
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	animated := &gif.GIF{Image: []*image.Paletted{frame}, Delay: []int{0}, Config: image.Config{Width: 4, Height: 3}}
	if err := gif.EncodeAll(file, animated); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeInspectionFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func syntheticWebP(width, height int) []byte {
	frame := []byte{0, 0, 0, 0x9d, 0x01, 0x2a, byte(width), byte(width >> 8), byte(height), byte(height >> 8)}
	data := make([]byte, 12+8+len(frame))
	copy(data, []byte("RIFF"))
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))
	copy(data[8:12], []byte("WEBP"))
	copy(data[12:16], []byte("VP8 "))
	binary.LittleEndian.PutUint32(data[16:20], uint32(len(frame)))
	copy(data[20:], frame)
	return data
}

func syntheticAVIF(width, height int, alpha bool) []byte {
	ftypPayload := append([]byte("avif"), make([]byte, 4)...)
	ftypPayload = append(ftypPayload, []byte("avif")...)
	ftyp := makeBMFFBox("ftyp", ftypPayload)
	ispe := make([]byte, 12)
	binary.BigEndian.PutUint32(ispe[4:8], uint32(width))
	binary.BigEndian.PutUint32(ispe[8:12], uint32(height))
	properties := makeBMFFBox("ispe", ispe)
	if alpha {
		properties = append(properties, makeBMFFBox("auxC", append(make([]byte, 4), []byte("alpha\x00")...))...)
	}
	ipco := makeBMFFBox("ipco", properties)
	iprp := makeBMFFBox("iprp", ipco)
	meta := makeBMFFBox("meta", append(make([]byte, 4), iprp...))
	return append(ftyp, meta...)
}

func syntheticKTX2(width, height, levels int) []byte {
	data := make([]byte, 80+levels*24+28)
	copy(data, ktx2Magic)
	binary.LittleEndian.PutUint32(data[20:24], uint32(width))
	binary.LittleEndian.PutUint32(data[24:28], uint32(height))
	binary.LittleEndian.PutUint32(data[40:44], uint32(levels))
	binary.LittleEndian.PutUint32(data[48:52], uint32(80+levels*24))
	binary.LittleEndian.PutUint32(data[52:56], 28)
	binary.LittleEndian.PutUint32(data[44:48], 2)
	for index := 0; index < levels; index++ {
		offset := 80 + index*24
		binary.LittleEndian.PutUint64(data[offset:offset+8], uint64(len(data)))
	}
	dfd := data[80+levels*24:]
	binary.LittleEndian.PutUint32(dfd[0:4], 28)
	binary.LittleEndian.PutUint16(dfd[10:12], 24)
	dfd[12], dfd[13], dfd[14] = 166, 1, 2
	return data
}

func syntheticGLB(t *testing.T, texture []byte) []byte {
	document := map[string]any{
		"asset":  map[string]string{"version": "2.0"},
		"meshes": []any{map[string]any{"primitives": []any{map[string]any{"attributes": map[string]int{"POSITION": 0}, "indices": 1}}}},
		"accessors": []any{
			map[string]any{"count": 4, "type": "VEC3", "min": []float64{-1, -2, -3}, "max": []float64{1, 2, 3}},
			map[string]any{"count": 6, "type": "SCALAR"},
		},
		"materials": []any{map[string]any{}}, "textures": []any{map[string]any{"source": 0}},
		"images":      []any{map[string]any{"bufferView": 0, "mimeType": "image/png"}},
		"bufferViews": []any{map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": len(texture)}},
		"buffers":     []any{map[string]any{"byteLength": len(texture)}}, "animations": []any{map[string]any{}},
	}
	jsonData, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	jsonData = append(jsonData, bytes.Repeat([]byte{' '}, (4-len(jsonData)%4)%4)...)
	bin := append([]byte(nil), texture...)
	bin = append(bin, bytes.Repeat([]byte{0}, (4-len(bin)%4)%4)...)
	length := 12 + 8 + len(jsonData) + 8 + len(bin)
	data := make([]byte, length)
	copy(data, []byte("glTF"))
	binary.LittleEndian.PutUint32(data[4:8], 2)
	binary.LittleEndian.PutUint32(data[8:12], uint32(length))
	position := 12
	binary.LittleEndian.PutUint32(data[position:position+4], uint32(len(jsonData)))
	binary.LittleEndian.PutUint32(data[position+4:position+8], 0x4e4f534a)
	copy(data[position+8:], jsonData)
	position += 8 + len(jsonData)
	binary.LittleEndian.PutUint32(data[position:position+4], uint32(len(bin)))
	binary.LittleEndian.PutUint32(data[position+4:position+8], 0x004e4942)
	copy(data[position+8:], bin)
	return data
}

func makeBMFFBox(name string, payload []byte) []byte {
	data := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(data[:4], uint32(len(data)))
	copy(data[4:8], name)
	copy(data[8:], payload)
	return data
}
