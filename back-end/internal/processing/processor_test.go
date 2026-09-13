package processing

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestProcessOutputsWritesDeterministicRoleImages(t *testing.T) {
	temporary := t.TempDir()
	sourcePath := filepath.Join(temporary, "source.png")
	writeFixtureImage(t, sourcePath, 8, 6)
	specs := []OutputSpec{
		{Role: "square", Path: filepath.Join(temporary, "square.png"), Width: 4, Height: 4},
		{Role: "wide", Path: filepath.Join(temporary, "wide.png"), Width: 4, Height: 3},
	}
	request := ExportRequest{
		Square: CropRect{X: 1, Y: 0, Width: 6, Height: 6},
		Wide:   CropRect{X: 0, Y: 0, Width: 8, Height: 6},
		Format: "png",
	}

	first, err := processOutputs(sourcePath, request, specs, "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := processOutputs(sourcePath, request, specs, "")
	if err != nil {
		t.Fatal(err)
	}
	if first[0].SHA256 != second[0].SHA256 || first[1].SHA256 != second[1].SHA256 {
		t.Fatalf("expected deterministic hashes, first=%v second=%v", first, second)
	}
	assertImageDimensions(t, specs[0].Path, 4, 4)
	assertImageDimensions(t, specs[1].Path, 4, 3)
}

func TestValidateCropRejectsWrongAspectAndUpscaling(t *testing.T) {
	spec := OutputSpec{Role: "wide", Width: 8, Height: 6}
	if err := validateCrop(image.Rect(0, 0, 16, 12), CropRect{Width: 8, Height: 8}, spec); err == nil {
		t.Fatal("expected wrong aspect ratio to fail")
	}
	if err := validateCrop(image.Rect(0, 0, 16, 12), CropRect{Width: 4, Height: 3}, spec); err == nil {
		t.Fatal("expected an upscaled crop to fail")
	}
}

func TestValidateEncodingOptions(t *testing.T) {
	valid := []ExportRequest{
		{Format: "png"},
		{Format: "webp", Quality: 95},
		{Format: "avif", Quality: 100, Lossless: true},
	}
	for _, request := range valid {
		if err := validateEncodingOptions(request); err != nil {
			t.Fatalf("expected %+v to be valid: %v", request, err)
		}
	}
	invalid := []ExportRequest{
		{Format: "jpeg", Quality: 95},
		{Format: "png", Quality: 95},
		{Format: "webp", Quality: 0},
		{Format: "avif", Quality: 101},
	}
	for _, request := range invalid {
		if err := validateEncodingOptions(request); err == nil {
			t.Fatalf("expected %+v to be invalid", request)
		}
	}
}

func TestProcessOutputsEncodesWebPAndAVIF(t *testing.T) {
	temporary := t.TempDir()
	sourcePath := filepath.Join(temporary, "source.png")
	writeFixtureImage(t, sourcePath, 8, 8)
	encoderDirectory, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	spec := OutputSpec{Role: "square", Path: filepath.Join(temporary, "square.png"), Width: 4, Height: 4}
	for _, format := range []string{"webp", "avif"} {
		request := ExportRequest{Square: CropRect{Width: 8, Height: 8}, Format: format, Quality: 95}
		outputs, err := processOutputs(sourcePath, request, []OutputSpec{spec}, encoderDirectory)
		if err != nil {
			t.Fatalf("encode %s: %v", format, err)
		}
		if outputs[0].Format != format || filepath.Ext(outputs[0].Path) != "."+format {
			t.Fatalf("expected %s measurement, got %+v", format, outputs[0])
		}
		assertEncodedFormat(t, outputs[0].Path, format)
	}
}

func writeFixtureImage(t *testing.T, path string, width, height int) {
	t.Helper()
	fixture := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			fixture.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 20), G: uint8(y * 30), B: uint8((x + y) * 10), A: 255})
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

func assertImageDimensions(t *testing.T, path string, width, height int) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoded, _, err := image.DecodeConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Width != width || decoded.Height != height {
		t.Fatalf("expected %dx%d image, got %dx%d", width, height, decoded.Width, decoded.Height)
	}
}

func assertEncodedFormat(t *testing.T, path, format string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	isWebP := len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
	isAVIF := len(data) >= 12 && string(data[4:12]) == "ftypavif"
	if (format == "webp" && !isWebP) || (format == "avif" && !isAVIF) {
		t.Fatalf("expected valid %s header", format)
	}
}
