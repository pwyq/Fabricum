package processing

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingNativeEncoderExplainsSetup(t *testing.T) {
	_, err := findNativeEncoder("cwebp", "webp", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "--encoder-directory") {
		t.Fatalf("expected actionable missing codec error, got %v", err)
	}
}

func TestNativeCodecsSupportLossyAndLosslessAlpha(t *testing.T) {
	for _, tool := range []string{"cwebp", "dwebp", "avifenc", "avifdec"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("native codec test tool %s is not installed", tool)
		}
	}
	temporary := t.TempDir()
	sourcePath := filepath.Join(temporary, "source.png")
	writeAlphaFixture(t, sourcePath)
	spec := OutputSpec{Role: "square", Path: filepath.Join(temporary, "square.png"), Width: 4, Height: 4}
	for _, sample := range []struct {
		format  string
		decoder string
	}{
		{format: "webp", decoder: "dwebp"},
		{format: "avif", decoder: "avifdec"},
	} {
		for _, lossless := range []bool{false, true} {
			request := ExportRequest{Square: CropRect{Width: 4, Height: 4}, Format: sample.format, Quality: 75, Lossless: lossless}
			first, err := PrepareOutputs(context.Background(), sourcePath, request, []OutputSpec{spec}, "")
			if err != nil {
				t.Fatalf("encode %s lossless=%t: %v", sample.format, lossless, err)
			}
			second, err := PrepareOutputs(context.Background(), sourcePath, request, []OutputSpec{spec}, "")
			if err != nil {
				t.Fatalf("repeat %s lossless=%t: %v", sample.format, lossless, err)
			}
			if !bytes.Equal(first[0].Data, second[0].Data) {
				t.Fatalf("%s lossless=%t output is not deterministic", sample.format, lossless)
			}
			if err := WriteOutputs(first); err != nil {
				t.Fatal(err)
			}
			assertEncodedFormat(t, first[0].Path, sample.format)
			decoded := decodeNativeTestOutput(t, sample.decoder, first[0].Path, temporary)
			transparent := color.NRGBAModel.Convert(decoded.At(0, 0)).(color.NRGBA)
			semiTransparent := color.NRGBAModel.Convert(decoded.At(1, 0)).(color.NRGBA)
			if transparent.A != 0 || semiTransparent.A == 0 {
				t.Fatalf("%s lossless=%t did not preserve alpha: transparent=%+v semi=%+v", sample.format, lossless, transparent, semiTransparent)
			}
			if sample.format == "webp" && lossless && (transparent.R != 17 || transparent.G != 29 || transparent.B != 43) {
				t.Fatalf("lossless WebP changed transparent RGB: %+v", transparent)
			}
		}
	}
}

func writeAlphaFixture(t *testing.T, path string) {
	t.Helper()
	fixture := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			fixture.SetNRGBA(x, y, color.NRGBA{R: uint8(40 + x), G: uint8(60 + y), B: 90, A: 255})
		}
	}
	fixture.SetNRGBA(0, 0, color.NRGBA{R: 17, G: 29, B: 43, A: 0})
	fixture.SetNRGBA(1, 0, color.NRGBA{R: 80, G: 90, B: 100, A: 128})
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

func decodeNativeTestOutput(t *testing.T, decoder, inputPath, directory string) image.Image {
	t.Helper()
	outputPath := filepath.Join(directory, decoder+"-decoded.png")
	var command *exec.Cmd
	if decoder == "dwebp" {
		command = exec.Command(decoder, "-quiet", inputPath, "-o", outputPath)
	} else {
		command = exec.Command(decoder, inputPath, outputPath)
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("decode %s: %v: %s", decoder, err, bytes.TrimSpace(output))
	}
	file, err := os.Open(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoded, _, err := image.Decode(file)
	if err != nil {
		t.Fatalf("read decoded %s: %v", decoder, err)
	}
	return decoded
}
