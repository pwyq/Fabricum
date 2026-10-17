package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"fabricum/back-end/internal/processing"
)

func TestJPEGOptimizationCLIFlags(t *testing.T) {
	for _, args := range [][]string{{}, {"--unknown"}, {"--strip-metadata"}} {
		if err := RunJPEGOptimizationArgs(nil, args, &bytes.Buffer{}); err == nil {
			t.Fatalf("expected argument error: %v", args)
		}
	}
	if err := RunJPEGOptimizationArgs(nil, []string{"--help"}, &bytes.Buffer{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatal(err)
	}
}

func TestJPEGOptimizationCLIReportsEveryFile(t *testing.T) {
	directory := jpegCLIToolDirectory(t)
	root := t.TempDir()
	path, invalid := filepath.Join(root, "valid.jpg"), filepath.Join(root, "invalid.jpg")
	pixels := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 64; x++ {
			pixels.Set(x, y, color.RGBA{R: uint8(x * 3), G: uint8(y * 5), B: 80, A: 255})
		}
	}
	var input bytes.Buffer
	if err := jpeg.Encode(&input, pixels, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, input.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(invalid, []byte("invalid JPEG"), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err := RunJPEGOptimizationArgs(context.Background(), []string{"--encoder-directory", directory, "--progressive", invalid, path}, &output)
	if err == nil {
		t.Fatal("expected batch failure")
	}
	var receipt JPEGOptimizationReceipt
	if err := json.Unmarshal(output.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.SchemaVersion != JPEGOptimizationReceiptSchemaVersion || receipt.Processor == "" || len(receipt.Files) != 2 {
		t.Fatalf("incomplete receipt: %+v", receipt)
	}
	if receipt.Files[0].Error == "" || receipt.Files[0].Changed || receipt.Files[1].Error != "" || !receipt.Files[1].Changed || !receipt.Files[1].Progressive {
		t.Fatalf("batch results: %+v", receipt.Files)
	}
	if receipt.Files[1].OptimizerVersion != processing.JPEGTranVersion || receipt.Files[1].SavingsBytes <= 0 || receipt.Files[1].OutputBytes >= receipt.Files[1].InputBytes {
		t.Fatalf("missing measurements: %+v", receipt.Files[1])
	}
	unchanged, _ := os.ReadFile(invalid)
	if string(unchanged) != "invalid JPEG" {
		t.Fatal("replaced invalid input")
	}
	// A path starting with a dash is accepted after --.
	if err := os.WriteFile(filepath.Join(root, "-valid.jpg"), input.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	output.Reset()
	if err := RunJPEGOptimizationArgs(nil, []string{"--encoder-directory", directory, "--", "-valid.jpg"}, &output); err != nil {
		t.Fatal(err)
	}
}

func jpegCLIToolDirectory(t *testing.T) string {
	t.Helper()
	if tool, err := exec.LookPath("jpegtran"); err == nil {
		return filepath.Dir(tool)
	}
	directory, _ := filepath.Abs("../../../bin/codecs")
	name := "jpegtran"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if _, err := os.Stat(filepath.Join(directory, name)); err == nil {
		return directory
	}
	if os.Getenv("CI") != "" {
		t.Fatal("CI must install pinned jpegtran")
	}
	t.Skip("install pinned jpegtran for CLI fixtures")
	return ""
}
