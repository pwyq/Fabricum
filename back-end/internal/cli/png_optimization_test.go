package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"fabricum/back-end/internal/processing"
)

func TestPNGOptimizationCLIFlags(t *testing.T) {
	for _, args := range [][]string{{}, {"--unknown"}} {
		if err := RunPNGOptimizationArgs(context.Background(), args, &bytes.Buffer{}); err == nil {
			t.Fatalf("expected argument error for %v", args)
		}
	}
	if err := RunPNGOptimizationArgs(context.Background(), []string{"--help"}, &bytes.Buffer{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help: %v", err)
	}
}

func TestPNGOptimizationCLIReportsEveryFile(t *testing.T) {
	directory := pngCLIToolDirectory(t)
	root := t.TempDir()
	path, invalid := filepath.Join(root, "valid.png"), filepath.Join(root, "invalid.png")
	pixels := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			pixels.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 8), B: 123, A: uint8((x % 3) * 127)})
		}
	}
	var input bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.NoCompression}).Encode(&input, pixels); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, input.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(invalid, []byte("invalid PNG"), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err := RunPNGOptimizationArgs(context.Background(), []string{"--encoder-directory", directory, "--strip-metadata", invalid, path}, &output)
	if err == nil {
		t.Fatal("expected batch failure")
	}
	var receipt PNGOptimizationReceipt
	if err := json.Unmarshal(output.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.SchemaVersion != PNGOptimizationReceiptSchemaVersion || receipt.Processor == "" || len(receipt.Files) != 2 {
		t.Fatalf("incomplete receipt: %+v", receipt)
	}
	if receipt.Files[0].Error == "" || receipt.Files[0].Changed || receipt.Files[1].Error != "" || !receipt.Files[1].Changed {
		t.Fatalf("batch results: %+v", receipt.Files)
	}
	if receipt.Files[1].OptimizerVersion != processing.OxipngVersion || receipt.Files[1].SavingsBytes <= 0 || receipt.Files[1].OutputBytes >= receipt.Files[1].InputBytes {
		t.Fatalf("missing measurements: %+v", receipt.Files[1])
	}
	unchanged, _ := os.ReadFile(invalid)
	if string(unchanged) != "invalid PNG" {
		t.Fatal("invalid input was replaced")
	}
}

func pngCLIToolDirectory(t *testing.T) string {
	t.Helper()
	if tool, err := exec.LookPath("oxipng"); err == nil {
		return filepath.Dir(tool)
	}
	directory, _ := filepath.Abs("../../../bin/codecs")
	name := "oxipng"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if _, err := os.Stat(filepath.Join(directory, name)); err == nil {
		return directory
	}
	if os.Getenv("CI") != "" {
		t.Fatal("CI must install pinned oxipng")
	}
	t.Skip("install pinned oxipng for CLI fixtures")
	return ""
}
