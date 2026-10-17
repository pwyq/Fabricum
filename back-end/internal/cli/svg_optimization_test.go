package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"fabricum/back-end/internal/processing"
)

func TestSVGOptimizationCLIFlags(t *testing.T) {
	for _, args := range [][]string{{}, {"--unknown"}, {"--precision", "2"}, {"--encoder-directory", "tools"}} {
		if err := RunSVGOptimizationArgs(nil, args, &bytes.Buffer{}); err == nil {
			t.Fatalf("expected argument error: %v", args)
		}
	}
	if err := RunSVGOptimizationArgs(nil, []string{"--help"}, &bytes.Buffer{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatal(err)
	}
}

func TestSVGOptimizationCLIBatch(t *testing.T) {
	root := t.TempDir()
	valid, unsafe := filepath.Join(root, "valid.svg"), filepath.Join(root, "unsafe.svg")
	input := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><!-- editor --><g><rect width="24" height="24"/></g></svg>`
	bad := `<svg xmlns="http://www.w3.org/2000/svg"><script/></svg>`
	for path, data := range map[string]string{valid: input, unsafe: bad, filepath.Join(root, "-icon.svg"): input} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	if err := RunSVGOptimizationArgs(context.Background(), []string{unsafe, valid}, &output); err == nil {
		t.Fatal("expected batch failure")
	}
	var receipt SVGOptimizationReceipt
	if err := json.Unmarshal(output.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.SchemaVersion != SVGOptimizationReceiptSchemaVersion || receipt.Processor == "" || len(receipt.Files) != 2 {
		t.Fatalf("incomplete receipt: %+v", receipt)
	}
	if receipt.Files[0].Error == "" || receipt.Files[0].Changed || receipt.Files[1].Error != "" || !receipt.Files[1].Changed || receipt.Files[1].SavingsBytes <= 0 || receipt.Files[1].OptimizerVersion != processing.SVGOptimizerVersion {
		t.Fatalf("batch results: %+v", receipt.Files)
	}
	unchanged, _ := os.ReadFile(unsafe)
	if string(unchanged) != bad {
		t.Fatal("replaced unsafe input")
	}
	t.Chdir(root)
	if err := RunSVGOptimizationArgs(nil, []string{"--", "-icon.svg"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	output.Reset()
	if err := RunSVGOptimizationArgs(ctx, []string{valid}, &output); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := json.Unmarshal(output.Bytes(), &receipt); err != nil || receipt.Files[0].Error == "" || receipt.Files[0].Changed {
		t.Fatalf("cancellation receipt: %+v, %v", receipt, err)
	}
}

type brokenSVGReceiptWriter struct{}

func (brokenSVGReceiptWriter) Write([]byte) (int, error) { return 0, errors.New("writer failed") }

func TestSVGReceiptWriteFailure(t *testing.T) {
	err := RunSVGOptimization(nil, []processing.SVGOptimizationRequest{{Path: "missing.svg"}}, brokenSVGReceiptWriter{})
	if err == nil {
		t.Fatal("receipt writer error lost")
	}
}
