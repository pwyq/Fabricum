package processing

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestTransformOperationsHaveObservablePixelsAndStableMeasurements(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.png")
	writeTransformFixture(t, source, 4, 2, func(x, y int) color.NRGBA {
		return color.NRGBA{R: uint8(20 + x*30), G: uint8(40 + y*50), B: uint8(60 + x + y), A: 255}
	})
	ao := filepath.Join(root, "ao.png")
	writeTransformFixture(t, ao, 2, 2, func(x, y int) color.NRGBA {
		return color.NRGBA{R: uint8(10 + x + y*2), G: 0, B: 0, A: 255}
	})
	roughness := filepath.Join(root, "roughness.png")
	writeTransformFixture(t, roughness, 2, 2, func(x, y int) color.NRGBA {
		return color.NRGBA{R: uint8(90 + x + y*3), G: 0, B: 0, A: 255}
	})

	fillResize := ResizeSpec{Width: 4, Height: 4, Fit: "fill", Filter: "nearest"}
	containResize := ResizeSpec{Width: 6, Height: 6, Fit: "contain", Filter: "nearest"}
	request := TransformRequest{
		Source: source,
		Constraints: SourceConstraints{
			Format: "png", Width: 4, Height: 2,
		},
		Format: "png",
		Outputs: []TransformOutputSpec{
			{Role: "crop", Path: filepath.Join(root, "crop.png"), Transform: ImageTransform{Crop: &CropRect{X: 1, Y: 0, Width: 2, Height: 2}}},
			{Role: "fill", Path: filepath.Join(root, "fill.png"), Transform: ImageTransform{Resize: &fillResize}},
			{Role: "contain", Path: filepath.Join(root, "contain.png"), Transform: ImageTransform{Resize: &containResize}},
			{Role: "padding", Path: filepath.Join(root, "padding.png"), Transform: ImageTransform{Padding: &PaddingSpec{Top: 1, Right: 2, Bottom: 3, Left: 4}}},
			{Role: "pack", Path: filepath.Join(root, "pack.png"), Transform: ImageTransform{
				Resize: &ResizeSpec{Width: 2, Height: 2, Fit: "fill", Filter: "nearest"},
				Pack: &ChannelPack{
					Red:   ChannelInput{Source: ao, Channel: "red"},
					Green: ChannelInput{Source: roughness, Channel: "red"},
					Blue:  ChannelInput{Constant: uint8Pointer(0)},
				},
			}},
		},
	}
	first, err := PrepareTransformOutputs(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PrepareTransformOutputs(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != len(second) {
		t.Fatalf("expected equal output counts, got %d and %d", len(first), len(second))
	}
	for index := range first {
		if !bytes.Equal(first[index].Data, second[index].Data) || first[index].Measurement.SHA256 != second[index].Measurement.SHA256 {
			t.Fatalf("output %d is not repeatable", index)
		}
	}

	assertTransformImage(t, first[0].Data, 2, 2, func(x, y int, pixel color.NRGBA) {
		want := color.NRGBA{R: uint8(50 + x*30), G: uint8(40 + y*50), B: uint8(61 + x + y), A: 255}
		if pixel != want {
			t.Fatalf("crop pixel (%d,%d) = %+v, want %+v", x, y, pixel, want)
		}
	})
	assertTransformImage(t, first[1].Data, 4, 4, func(x, y int, pixel color.NRGBA) {
		if pixel != (color.NRGBA{R: uint8(20 + x*30), G: uint8(40 + (y/2)*50), B: uint8(60 + x + y/2), A: 255}) {
			t.Fatalf("fill pixel (%d,%d) = %+v", x, y, pixel)
		}
	})
	assertTransformImage(t, first[2].Data, 6, 6, func(x, y int, pixel color.NRGBA) {
		if (x == 0 && y == 0 && pixel.A != 0) || (x == 2 && y == 1 && pixel.A == 0) {
			t.Fatalf("contain padding pixel (%d,%d) = %+v", x, y, pixel)
		}
	})
	assertTransformImage(t, first[3].Data, 10, 6, func(x, y int, pixel color.NRGBA) {
		if (x < 4 || x >= 8 || y == 0 || y >= 3) && pixel.A != 0 {
			t.Fatalf("padding pixel (%d,%d) = %+v", x, y, pixel)
		}
	})
	assertTransformImage(t, first[4].Data, 2, 2, func(x, y int, pixel color.NRGBA) {
		want := color.NRGBA{R: uint8(10 + x + y*2), G: uint8(90 + x + y*3), B: 0, A: 255}
		if pixel != want {
			t.Fatalf("packed pixel (%d,%d) = %+v, want %+v", x, y, pixel, want)
		}
	})
	if first[4].Measurement.HasAlpha || first[4].Measurement.Filter != "nearest" {
		t.Fatalf("unexpected packed measurement: %+v", first[4].Measurement)
	}

	if _, err := Transform(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	for _, output := range first {
		data, err := os.ReadFile(output.Path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, output.Data) {
			t.Fatalf("written %s differs from prepared bytes", output.Path)
		}
	}
}

func TestTransformChannelAndAlphaOperations(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.png")
	writeTransformFixture(t, source, 2, 1, func(x, _ int) color.NRGBA {
		if x == 0 {
			return color.NRGBA{R: 200, G: 100, B: 50, A: 0}
		}
		return color.NRGBA{R: 10, G: 20, B: 30, A: 128}
	})
	request := TransformRequest{
		Source: source, Format: "png",
		Outputs: []TransformOutputSpec{
			{Role: "gray", Path: filepath.Join(root, "gray.png"), Transform: ImageTransform{Grayscale: true, RemoveAlpha: true}},
			{Role: "alpha", Path: filepath.Join(root, "alpha.png"), Transform: ImageTransform{Channel: "alpha"}},
		},
	}
	outputs, err := PrepareTransformOutputs(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	assertTransformImage(t, outputs[0].Data, 2, 1, func(x, _ int, pixel color.NRGBA) {
		want := []uint8{124, 18}[x]
		if pixel.A != 255 || pixel.R != want || pixel.G != want || pixel.B != want {
			t.Fatalf("gray pixel = %+v, want %d", pixel, want)
		}
	})
	assertTransformImage(t, outputs[1].Data, 2, 1, func(x, _ int, pixel color.NRGBA) {
		want := []uint8{0, 128}[x]
		if pixel.R != want || pixel.G != want || pixel.B != want || pixel.A != 255 {
			t.Fatalf("alpha channel pixel = %+v, want %d", pixel, want)
		}
	})
}

func TestTransformIndexedPNGIsDeterministicAndPaletteEncoded(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.png")
	writeTransformFixture(t, source, 32, 32, func(x, y int) color.NRGBA {
		return color.NRGBA{
			R: uint8(x * 8),
			G: uint8(y * 8),
			B: uint8((x + y) * 4),
			A: uint8((x*3 + y*5) % 256),
		}
	})
	request := TransformRequest{
		Source: source, Format: "png", PNGMode: PNGModeIndexed,
		Outputs: []TransformOutputSpec{{Role: "sprite", Path: filepath.Join(root, "sprite.png")}},
	}
	first, err := PrepareTransformOutputs(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PrepareTransformOutputs(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first[0].Data, second[0].Data) {
		t.Fatal("indexed PNG output is not deterministic")
	}
	decoded, err := png.Decode(bytes.NewReader(first[0].Data))
	if err != nil {
		t.Fatal(err)
	}
	paletted, ok := decoded.(*image.Paletted)
	if !ok || len(paletted.Palette) > 256 {
		t.Fatalf("decoded indexed PNG = %T with %d colors", decoded, len(paletted.Palette))
	}
	var absoluteError int
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			want := color.NRGBA{R: uint8(x * 8), G: uint8(y * 8), B: uint8((x + y) * 4), A: uint8((x*3 + y*5) % 256)}
			got := color.NRGBAModel.Convert(decoded.At(x, y)).(color.NRGBA)
			for _, difference := range []int{
				int(got.R)*int(got.A)/255 - int(want.R)*int(want.A)/255,
				int(got.G)*int(got.A)/255 - int(want.G)*int(want.A)/255,
				int(got.B)*int(got.A)/255 - int(want.B)*int(want.A)/255,
				int(got.A) - int(want.A),
			} {
				absoluteError += max(difference, -difference)
			}
		}
	}
	if meanError := float64(absoluteError) / (32 * 32 * 4); meanError > 4 {
		t.Fatalf("indexed PNG mean premultiplied RGBA error = %.3f, want <= 4", meanError)
	}
}

func TestTransformRejectsInvalidSourceAndOperations(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.png")
	writeTransformFixture(t, source, 2, 2, func(_, _ int) color.NRGBA { return color.NRGBA{A: 255} })
	cases := []TransformRequest{
		{Source: source, Format: "png", Constraints: SourceConstraints{Format: "jpeg"}, Outputs: []TransformOutputSpec{{Path: filepath.Join(root, "out.png")}}},
		{Source: source, Format: "png", Constraints: SourceConstraints{Width: 3, Height: 3}, Outputs: []TransformOutputSpec{{Path: filepath.Join(root, "out.png")}}},
		{Source: source, Format: "png", Outputs: []TransformOutputSpec{{Path: filepath.Join(root, "out.png"), Transform: ImageTransform{Resize: &ResizeSpec{Width: 1, Height: 1, Fit: "cover"}}}}},
		{Source: source, Format: "png", PNGMode: "unknown", Outputs: []TransformOutputSpec{{Path: filepath.Join(root, "out.png")}}},
		{Source: source, Format: "webp", Quality: 90, PNGMode: PNGModeIndexed, Outputs: []TransformOutputSpec{{Path: filepath.Join(root, "out.webp")}}},
	}
	for _, request := range cases {
		if _, err := PrepareTransformOutputs(context.Background(), request); err == nil {
			t.Fatalf("accepted invalid request: %+v", request)
		}
	}
}

func writeTransformFixture(t *testing.T, path string, width, height int, pixel func(int, int) color.NRGBA) {
	t.Helper()
	imageData := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			imageData.SetNRGBA(x, y, pixel(x, y))
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, imageData); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func assertTransformImage(t *testing.T, data []byte, width, height int, check func(int, int, color.NRGBA)) {
	t.Helper()
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != width || decoded.Bounds().Dy() != height {
		t.Fatalf("got %dx%d image, want %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy(), width, height)
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			check(x, y, color.NRGBAModel.Convert(decoded.At(x, y)).(color.NRGBA))
		}
	}
}

func uint8Pointer(value uint8) *uint8 {
	return &value
}
