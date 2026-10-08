package processing

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPNGOptimizationNativeFixtures(t *testing.T) {
	directory := pngTestToolDirectory(t)
	for name, input := range pngOptimizationFixtures(t) {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name+".png")
			writePNGTestFile(t, path, input)
			request := PNGOptimizationRequest{Path: path, EncoderDirectory: directory}
			result, err := OptimizePNG(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if !result.Changed || result.SavingsBytes <= 0 || result.InputBytes != len(input) || result.OutputBytes >= len(input) || result.SavingsPercent <= 0 {
				t.Fatalf("expected actual reduction: %+v", result)
			}
			if result.Optimizer != "oxipng" || result.OptimizerVersion != OxipngVersion || result.Format != "png" || result.ColorType == "" || result.BitDepth == 0 || result.MetadataStripped {
				t.Fatalf("incomplete receipt: %+v", result)
			}
			output, _ := os.ReadFile(path)
			if info, err := os.Stat(path); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
				t.Fatalf("file permissions changed: %v", err)
			}
			assertPNGLossless(t, input, output, false)
			oldTime := time.Unix(1234567890, 0)
			if err := os.Chtimes(path, oldTime, oldTime); err != nil {
				t.Fatal(err)
			}
			again, err := OptimizePNG(context.Background(), request)
			if err != nil || again.Changed || again.OutputBytes != result.OutputBytes || again.SavingsBytes != 0 {
				t.Fatalf("repeat rewrote or grew PNG: %+v, %v", again, err)
			}
			repeated, _ := os.ReadFile(path)
			info, _ := os.Stat(path)
			if !bytes.Equal(output, repeated) || !info.ModTime().Equal(oldTime) {
				t.Fatal("repeat changed bytes or timestamp")
			}
		})
	}
}

func TestPNGOptimizationExplicitMetadataStripping(t *testing.T) {
	directory := pngTestToolDirectory(t)
	input := pngOptimizationFixtures(t)["icc-profile"]
	input = insertPNGTestChunks(t, input, pngTestChunk("tEXt", append([]byte("Comment\x00"), bytes.Repeat([]byte("metadata "), 128)...)), pngTestChunk("tIME", []byte{7, 234, 10, 7, 12, 30, 0}))
	path := filepath.Join(t.TempDir(), "profile.png")
	writePNGTestFile(t, path, input)
	result, err := OptimizePNG(context.Background(), PNGOptimizationRequest{Path: path, EncoderDirectory: directory, StripMetadata: true})
	if err != nil || !result.Changed || !result.MetadataStripped {
		t.Fatalf("strip result: %+v, %v", result, err)
	}
	output, _ := os.ReadFile(path)
	assertPNGLossless(t, input, output, true)
}

func TestPNGOptimizationImprovesGoBestCompression(t *testing.T) {
	directory := pngTestToolDirectory(t)
	pixels, _ := png.Decode(bytes.NewReader(pngOptimizationFixtures(t)["rgb"]))
	var input bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&input, pixels); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "best-compression.png")
	writePNGTestFile(t, path, input.Bytes())
	result, err := OptimizePNG(context.Background(), PNGOptimizationRequest{Path: path, EncoderDirectory: directory})
	if err != nil || !result.Changed || result.SavingsBytes <= 0 {
		t.Fatalf("expected improvement over Go BestCompression: %+v, %v", result, err)
	}
	output, _ := os.ReadFile(path)
	assertPNGLossless(t, input.Bytes(), output, false)
	t.Logf("Go BestCompression: %d bytes; oxipng: %d bytes (%.1f%% smaller)", result.InputBytes, result.OutputBytes, result.SavingsPercent)
}

func TestPNGOptimizationFailuresRetainOriginal(t *testing.T) {
	input := pngOptimizationFixtures(t)["rgba-transparent"]
	changed := pngOptimizationFixtures(t)["rgba-changed"]
	hiddenChanged := pngOptimizationFixtures(t)["rgba-hidden-changed"]
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, sample := range []struct {
		name string
		data []byte
		ctx  context.Context
		run  pngOptimizer
	}{
		{"invalid-input", []byte("not a PNG"), context.Background(), nil},
		{"bad-checksum", append([]byte(nil), input...), context.Background(), nil},
		{"tool-failure", input, context.Background(), func(context.Context, []byte, string) ([]byte, string, error) {
			return nil, OxipngVersion, errors.New("optimizer failed")
		}},
		{"invalid-output", input, context.Background(), pngTestOptimizer([]byte("corrupt"))},
		{"changed-pixels", input, context.Background(), pngTestOptimizer(changed)},
		{"changed-hidden-rgb", input, context.Background(), pngTestOptimizer(hiddenChanged)},
		{"canceled", input, canceled, nil},
		{"apng", insertPNGTestChunks(t, input, pngTestChunk("acTL", []byte{0, 0, 0, 1, 0, 0, 0, 0})), context.Background(), nil},
		{"unsafe-chunk", insertPNGTestChunks(t, input, pngTestChunk("vpAG", []byte{1})), context.Background(), nil},
		{"trailing-data", append(append([]byte(nil), input...), 0), context.Background(), nil},
	} {
		t.Run(sample.name, func(t *testing.T) {
			if sample.name == "bad-checksum" {
				sample.data[20] ^= 1
			}
			path := filepath.Join(t.TempDir(), "input.png")
			writePNGTestFile(t, path, sample.data)
			_, err := optimizePNG(sample.ctx, PNGOptimizationRequest{Path: path}, sample.run)
			if err == nil {
				t.Fatal("expected failure")
			}
			output, _ := os.ReadFile(path)
			if !bytes.Equal(sample.data, output) {
				t.Fatal("failure changed original")
			}
		})
	}
}

func TestPNGOptimizationLargerAndEqualCandidatesRetainTimestamp(t *testing.T) {
	uncompressed := pngOptimizationFixtures(t)["rgb"]
	pixels, _ := png.Decode(bytes.NewReader(uncompressed))
	var encoded bytes.Buffer
	_ = (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&encoded, pixels)
	input := encoded.Bytes()
	for _, candidate := range [][]byte{input, uncompressed} {
		path := filepath.Join(t.TempDir(), "input.png")
		writePNGTestFile(t, path, input)
		oldTime := time.Unix(1234567890, 0)
		_ = os.Chtimes(path, oldTime, oldTime)
		result, err := optimizePNG(context.Background(), PNGOptimizationRequest{Path: path}, pngTestOptimizer(candidate))
		if err != nil || result.Changed || result.OutputBytes != len(input) {
			t.Fatalf("no-op result: %+v, %v", result, err)
		}
		info, _ := os.Stat(path)
		if !info.ModTime().Equal(oldTime) {
			t.Fatal("no-op changed timestamp")
		}
	}
}

func TestPNGOptimizationConcurrentEditIsRetained(t *testing.T) {
	input := pngOptimizationFixtures(t)["rgb"]
	path := filepath.Join(t.TempDir(), "input.png")
	writePNGTestFile(t, path, input)
	edited := []byte("concurrent edit")
	runner := func(context.Context, []byte, string) ([]byte, string, error) {
		writePNGTestFile(t, path, edited)
		decoded, _ := png.Decode(bytes.NewReader(input))
		var output bytes.Buffer
		_ = (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&output, decoded)
		return output.Bytes(), OxipngVersion, nil
	}
	_, err := optimizePNG(context.Background(), PNGOptimizationRequest{Path: path}, runner)
	if err == nil || !strings.Contains(err.Error(), "changed during optimization") {
		t.Fatalf("expected concurrent edit diagnostic: %v", err)
	}
	output, _ := os.ReadFile(path)
	if !bytes.Equal(output, edited) {
		t.Fatal("overwrote concurrent edit")
	}
}

func TestPNGAtomicReplacementFailureRetainsOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "original.png")
	writePNGTestFile(t, path, []byte("original"))
	if err := renameOptimizedPNG(path+".missing", path); err == nil {
		t.Fatal("expected rename failure")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "original" {
		t.Fatal("failed replacement lost original")
	}
}

func pngTestOptimizer(data []byte) pngOptimizer {
	return func(context.Context, []byte, string) ([]byte, string, error) { return data, OxipngVersion, nil }
}

func writePNGTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func pngTestToolDirectory(t *testing.T) string {
	t.Helper()
	if tool, err := exec.LookPath("oxipng"); err == nil {
		return filepath.Dir(tool)
	}
	directory, _ := filepath.Abs("../../../bin/codecs")
	if _, err := os.Stat(nativeToolPath(directory, "oxipng")); err == nil {
		return directory
	}
	if os.Getenv("CI") != "" {
		t.Fatal("CI must install pinned oxipng")
	}
	t.Skip("install pinned oxipng for native PNG optimization fixtures")
	return ""
}

func assertPNGLossless(t *testing.T, input, output []byte, stripped bool) {
	t.Helper()
	a, err := decodeOptimizationPNG(input)
	if err != nil {
		t.Fatal(err)
	}
	b, err := decodeOptimizationPNG(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := samePNGPixelValues(context.Background(), a.pixels, b.pixels); err != nil {
		t.Fatal(err)
	}
	var before, after []byte
	for _, chunk := range a.chunks {
		if chunk.kind != "IDAT" && !(stripped && (chunk.kind == "tEXt" || chunk.kind == "zTXt" || chunk.kind == "iTXt" || chunk.kind == "tIME")) {
			before = append(before, chunk.raw...)
		}
	}
	for _, chunk := range b.chunks {
		if chunk.kind != "IDAT" {
			after = append(after, chunk.raw...)
		}
	}
	if !bytes.Equal(before, after) {
		t.Fatal("changed preserved chunks or their order")
	}
}

func pngOptimizationFixtures(t *testing.T) map[string][]byte {
	t.Helper()
	rgb, rgba, changed := image.NewNRGBA(image.Rect(0, 0, 32, 24)), image.NewNRGBA(image.Rect(0, 0, 32, 24)), image.NewNRGBA(image.Rect(0, 0, 32, 24))
	gray, gray16 := image.NewGray(rgb.Bounds()), image.NewGray16(rgb.Bounds())
	rgba16 := image.NewNRGBA64(rgb.Bounds())
	indexed := image.NewPaletted(rgb.Bounds(), color.Palette{color.NRGBA{R: 17, G: 29, B: 43, A: 0}, color.NRGBA{R: 70, G: 80, B: 90, A: 128}, color.NRGBA{R: 100, G: 150, B: 200, A: 255}})
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			pixel := color.NRGBA{R: uint8(x * 7), G: uint8(y * 9), B: 80, A: 255}
			rgb.SetNRGBA(x, y, pixel)
			pixel.A = []uint8{0, 127, 255}[x%3]
			rgba.SetNRGBA(x, y, pixel)
			pixel.R ^= 1
			changed.SetNRGBA(x, y, pixel)
			gray.SetGray(x, y, color.Gray{Y: uint8(x * 7)})
			gray16.SetGray16(x, y, color.Gray16{Y: uint16(x*1700 + y)})
			rgba16.SetNRGBA64(x, y, color.NRGBA64{R: uint16(x*1000 + 5), G: 12345, B: 54321, A: uint16((x % 3) * 32000)})
			indexed.SetColorIndex(x, y, uint8(x%3))
		}
	}
	hiddenChanged := image.NewNRGBA(rgba.Bounds())
	copy(hiddenChanged.Pix, rgba.Pix)
	hiddenChanged.Pix[0] ^= 1 // Change only RGB of a fully transparent pixel.
	result := make(map[string][]byte)
	for name, source := range map[string]image.Image{"rgb": rgb, "rgba-transparent": rgba, "rgba-changed": changed, "rgba-hidden-changed": hiddenChanged, "grayscale": gray, "gray16": gray16, "rgba16-transparent": rgba16, "indexed": indexed} {
		var data bytes.Buffer
		if err := (&png.Encoder{CompressionLevel: png.NoCompression}).Encode(&data, source); err != nil {
			t.Fatal(err)
		}
		result[name] = data.Bytes()
	}
	addPNGProfileFixtures(t, result)
	return result
}
