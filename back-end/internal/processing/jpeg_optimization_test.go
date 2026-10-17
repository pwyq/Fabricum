package processing

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestJPEGOptimizationNativeFixtures(t *testing.T) {
	directory := jpegTestToolDirectory(t)
	for _, gray := range []bool{false, true} {
		for _, progressive := range []bool{false, true} {
			name := fmtJPEGFixture(gray, progressive)
			t.Run(name, func(t *testing.T) {
				input := jpegOptimizationFixture(t, gray)
				if progressive {
					workspace := t.TempDir()
					path := filepath.Join(workspace, "source.jpg")
					writePNGTestFile(t, path, input)
					var err error
					input, err = transcodeJPEG(context.Background(), nativeToolPath(directory, "jpegtran"), workspace, path, "progressive.jpg", true)
					if err != nil {
						t.Fatal(err)
					}
					// Legal marker fill ensures a reduction even for an already
					// entropy-optimized progressive fixture.
					position := bytes.Index(input, []byte{0xff, 0xdb})
					input = append(append(append([]byte{}, input[:position]...), bytes.Repeat([]byte{0xff}, 512)...), input[position:]...)
				}
				input = jpegWithMetadata(t, input)
				// Metadata after entropy data must survive as well as headers.
				comment := insertJPEGMarker([]byte{0xff, 0xd8, 0xff, 0xd9}, 0xfe, []byte("comment after scan"))
				input = append(append(append([]byte{}, input[:len(input)-2]...), comment[2:len(comment)-2]...), 0xff, 0xd9)
				path := filepath.Join(t.TempDir(), "image with spaces.jpg")
				writePNGTestFile(t, path, input)
				result, err := OptimizeJPEG(nil, JPEGOptimizationRequest{Path: path, EncoderDirectory: directory})
				if err != nil {
					t.Fatal(err)
				}
				if !result.Changed || result.InputBytes != len(input) || result.OutputBytes >= result.InputBytes || result.SavingsBytes != result.InputBytes-result.OutputBytes || result.SavingsPercent <= 0 || result.Optimizer != "jpegtran" || result.OptimizerVersion != JPEGTranVersion || result.Format != "jpeg" || result.Progressive != progressive {
					t.Fatalf("incomplete result: %+v", result)
				}
				output, _ := os.ReadFile(path)
				assertJPEGLossless(t, input, output, directory)
				info, _ := os.Stat(path)
				if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
					t.Fatal("permissions changed")
				}
				old := time.Unix(1234567890, 0)
				if err := os.Chtimes(path, old, old); err != nil {
					t.Fatal(err)
				}
				again, err := OptimizeJPEG(context.Background(), JPEGOptimizationRequest{Path: path, EncoderDirectory: directory})
				if err != nil || again.Changed || again.OutputBytes != len(output) || again.SavingsBytes != 0 {
					t.Fatalf("repeat: %+v, %v", again, err)
				}
				repeated, _ := os.ReadFile(path)
				info, _ = os.Stat(path)
				if !bytes.Equal(output, repeated) || !info.ModTime().Equal(old) {
					t.Fatal("no-op rewrote bytes or timestamp")
				}
			})
		}
	}
}

func TestJPEGOptimizationProgressiveOption(t *testing.T) {
	directory := jpegTestToolDirectory(t)
	input := jpegOptimizationFixture(t, false)
	path := filepath.Join(t.TempDir(), "input.jpg")
	writePNGTestFile(t, path, input)
	result, err := OptimizeJPEG(context.Background(), JPEGOptimizationRequest{Path: path, EncoderDirectory: directory, Progressive: true})
	if err != nil || !result.Changed || !result.Progressive {
		t.Fatalf("progressive: %+v, %v", result, err)
	}
	output, _ := os.ReadFile(path)
	assertJPEGLossless(t, input, output, directory)
}

func TestJPEGInvalidEntropyRetainsOriginal(t *testing.T) {
	directory := jpegTestToolDirectory(t)
	input := jpegOptimizationFixture(t, false)
	parsed, err := parseOptimizationJPEG(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"missing-tables", "missing-entropy"} {
		t.Run(mode, func(t *testing.T) {
			bad := []byte{0xff, 0xd8}
			for _, part := range parsed.parts {
				if mode == "missing-tables" && part.marker == 0xc4 || mode == "missing-entropy" && part.marker == 0 {
					continue
				}
				bad = append(bad, part.raw...)
			}
			path := filepath.Join(t.TempDir(), "invalid.jpg")
			writePNGTestFile(t, path, bad)
			result, err := OptimizeJPEG(nil, JPEGOptimizationRequest{Path: path, EncoderDirectory: directory})
			if err == nil || result.Changed {
				t.Fatalf("accepted corrupt entropy: %+v, %v", result, err)
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(bad, after) {
				t.Fatal("corrupt input replaced")
			}
		})
	}
}

func TestJPEGOptimizationFailuresAndNoOpsRetainOriginal(t *testing.T) {
	input := jpegOptimizationFixture(t, false)
	for _, sample := range []struct {
		name            string
		data, candidate []byte
		failure         bool
	}{
		{"invalid-input", []byte("not jpeg"), nil, true},
		{"truncated", input[:len(input)-2], nil, true},
		{"trailing", append(append([]byte{}, input...), 0), nil, true},
		{"mpf", insertJPEGMarker(input, 0xe2, []byte("MPF\x00offsets")), nil, true},
		{"jumbf", insertJPEGMarker(input, 0xeb, []byte("JPsignature")), nil, true},
		{"tool-failure", input, nil, true},
		{"invalid-output", input, []byte("bad output"), true},
		{"changed-metadata", jpegWithMetadata(t, input), input, true},
		{"equal", input, input, false},
		{"larger", input, append(append([]byte{0xff, 0xd8}, bytes.Repeat([]byte{0xff}, 100)...), input[2:]...), false},
	} {
		t.Run(sample.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input.jpg")
			writePNGTestFile(t, path, sample.data)
			old := time.Unix(1234567890, 0)
			_ = os.Chtimes(path, old, old)
			runner := func(context.Context, []byte, string, bool) ([]byte, string, error) {
				if sample.candidate == nil {
					return nil, JPEGTranVersion, errors.New("optimizer failure")
				}
				return sample.candidate, JPEGTranVersion, nil
			}
			result, err := optimizeJPEG(context.Background(), JPEGOptimizationRequest{Path: path}, runner)
			if (err != nil) != sample.failure || result.Changed || result.OutputBytes != len(sample.data) {
				t.Fatalf("unexpected result: %+v, %v", result, err)
			}
			output, _ := os.ReadFile(path)
			info, _ := os.Stat(path)
			if !bytes.Equal(sample.data, output) || !info.ModTime().Equal(old) {
				t.Fatal("failure/no-op changed original")
			}
		})
	}
}

func TestJPEGOptimizationConcurrentEditAndCancellation(t *testing.T) {
	directory := jpegTestToolDirectory(t)
	input := jpegOptimizationFixture(t, false)
	candidate, _, err := runJPEGOptimizer(context.Background(), input, directory, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, cancelDuring := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "input.jpg")
		writePNGTestFile(t, path, input)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		expected := input
		runner := func(context.Context, []byte, string, bool) ([]byte, string, error) {
			if cancelDuring {
				cancel()
			} else {
				expected = []byte("concurrent edit")
				writePNGTestFile(t, path, expected)
			}
			return candidate, JPEGTranVersion, nil
		}
		result, err := optimizeJPEG(ctx, JPEGOptimizationRequest{Path: path}, runner)
		if err == nil || result.Changed {
			t.Fatalf("expected failure: %+v, %v", result, err)
		}
		output, _ := os.ReadFile(path)
		if !bytes.Equal(output, expected) {
			t.Fatal("overwrote original or concurrent edit")
		}
	}
}

func TestJPEGOptimizationUnsupportedFramesAndPaths(t *testing.T) {
	input := jpegOptimizationFixture(t, false)
	parts, _ := parseOptimizationJPEG(input)
	var frame []byte
	for _, part := range parts.parts {
		if part.marker == 0xc0 {
			frame = part.raw
		}
	}
	for _, mutate := range []func([]byte){
		func(b []byte) { b[1] = 0xc3 }, // lossless predictive JPEG
		func(b []byte) { b[4] = 12 },
		func(b []byte) { b[9] = 4 }, // CMYK/YCCK
		func(b []byte) { b[5], b[6], b[7], b[8] = 0xff, 0xff, 0xff, 0xff },
	} {
		bad := append([]byte{}, frame...)
		mutate(bad)
		if _, err := parseOptimizationJPEG(bytes.Replace(input, frame, bad, 1)); err == nil {
			t.Fatal("accepted unsupported frame")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := OptimizeJPEG(ctx, JPEGOptimizationRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, path := range []string{"", filepath.Join(t.TempDir(), "missing"), t.TempDir()} {
		if _, err := OptimizeJPEG(nil, JPEGOptimizationRequest{Path: path}); err == nil {
			t.Fatalf("accepted path %q", path)
		}
	}
	path := filepath.Join(t.TempDir(), "link.jpg")
	target := filepath.Join(t.TempDir(), "input.jpg")
	writePNGTestFile(t, target, input)
	if err := os.Symlink(target, path); err == nil {
		if _, err := OptimizeJPEG(nil, JPEGOptimizationRequest{Path: path}); err == nil {
			t.Fatal("accepted symbolic link")
		}
	}
}

func jpegOptimizationFixture(t *testing.T, gray bool) []byte {
	t.Helper()
	pixels := image.NewNRGBA(image.Rect(0, 0, 67, 49))
	for y := 0; y < 49; y++ {
		for x := 0; x < 67; x++ {
			pixels.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 3), G: uint8(y * 5), B: uint8((x/9 + y/7) * 25), A: 255})
		}
	}
	var source image.Image = pixels
	if gray {
		g := image.NewGray(pixels.Bounds())
		for y := 0; y < 49; y++ {
			for x := 0; x < 67; x++ {
				g.Set(x, y, pixels.At(x, y))
			}
		}
		source = g
	}
	var output bytes.Buffer
	if err := jpeg.Encode(&output, source, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func fmtJPEGFixture(gray, progressive bool) string {
	name := "color"
	if gray {
		name = "gray"
	}
	if progressive {
		name += "-progressive"
	}
	return name
}

func insertJPEGMarker(input []byte, marker byte, body []byte) []byte {
	segment := []byte{0xff, marker, 0, 0}
	binary.BigEndian.PutUint16(segment[2:], uint16(len(body)+2))
	output := append([]byte{}, input[:2]...)
	output = append(output, segment...)
	output = append(output, body...)
	return append(output, input[2:]...)
}

func jpegWithMetadata(t *testing.T, input []byte) []byte {
	t.Helper()
	// A little-endian EXIF IFD with orientation=6; its offsets stay internal.
	exif := []byte("Exif\x00\x00II\x2a\x00\x08\x00\x00\x00\x01\x00\x12\x01\x03\x00\x01\x00\x00\x00\x06\x00\x00\x00\x00\x00\x00\x00")
	input = insertJPEGMarker(input, 0xe1, exif)
	input = insertJPEGMarker(input, 0xe1, []byte("http://ns.adobe.com/xap/1.0/\x00<x:xmpmeta>orientation 6</x:xmpmeta>"))
	profile := syntheticRGBProfile()
	input = insertJPEGMarker(input, 0xe2, append([]byte("ICC_PROFILE\x00\x02\x02"), profile[len(profile)/2:]...))
	input = insertJPEGMarker(input, 0xe2, append([]byte("ICC_PROFILE\x00\x01\x02"), profile[:len(profile)/2]...))
	input = insertJPEGMarker(input, 0xe0, []byte{'J', 'F', 'I', 'F', 0, 1, 2, 1, 0, 72, 0, 72, 1, 1, 20, 30, 40})
	return insertJPEGMarker(input, 0xfe, []byte("preserved comment"))
}

func assertJPEGLossless(t *testing.T, input, output []byte, directory string) {
	t.Helper()
	a, err := parseOptimizationJPEG(input)
	if err != nil {
		t.Fatal(err)
	}
	b, err := parseOptimizationJPEG(output)
	if err != nil || !sameJPEGMetadata(a, b) {
		t.Fatalf("metadata changed: %v", err)
	}
	before, err := jpeg.Decode(bytes.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	after, err := jpeg.Decode(bytes.NewReader(output))
	if err != nil {
		t.Fatal(err)
	}
	if before.Bounds() != after.Bounds() {
		t.Fatal("dimensions changed")
	}
	for y := 0; y < before.Bounds().Dy(); y++ {
		for x := 0; x < before.Bounds().Dx(); x++ {
			if color.NRGBAModel.Convert(before.At(x, y)) != color.NRGBAModel.Convert(after.At(x, y)) {
				t.Fatal("decoded samples changed")
			}
		}
	}
	workspace := t.TempDir()
	path := filepath.Join(workspace, "source.jpg")
	writePNGTestFile(t, path, input)
	first, err := transcodeJPEG(context.Background(), nativeToolPath(directory, "jpegtran"), workspace, path, "first.jpg", false)
	if err != nil {
		t.Fatal(err)
	}
	writePNGTestFile(t, path, output)
	second, err := transcodeJPEG(context.Background(), nativeToolPath(directory, "jpegtran"), workspace, path, "second.jpg", false)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("DCT coefficients/quantization changed: %v", err)
	}
}

func jpegTestToolDirectory(t *testing.T) string {
	t.Helper()
	if tool, err := exec.LookPath("jpegtran"); err == nil {
		return filepath.Dir(tool)
	}
	directory, _ := filepath.Abs("../../../bin/codecs")
	if _, err := os.Stat(nativeToolPath(directory, "jpegtran")); err == nil {
		return directory
	}
	if os.Getenv("CI") != "" {
		t.Fatal("CI must install pinned jpegtran")
	}
	t.Skip("install pinned jpegtran for native JPEG fixtures")
	return ""
}
