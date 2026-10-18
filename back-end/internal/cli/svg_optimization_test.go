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
	for _, args := range [][]string{{}, {"--unknown"}, {"--precision", "2"}, {"--precision", "-1", "icon.svg"}, {"--precision", "9", "icon.svg"}, {"--output", "out.svg", "a.svg", "b.svg"}, {"--encoder-directory", "tools"}} {
		if err := RunSVGOptimizationArgs(nil, args, &bytes.Buffer{}); err == nil {
			t.Fatalf("expected argument error: %v", args)
		}
	}
	if err := RunSVGOptimizationArgs(nil, []string{"--help"}, &bytes.Buffer{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatal(err)
	}
}

func TestSVGIconCLIOutput(t *testing.T) {
	directory := t.TempDir()
	source, destination := filepath.Join(directory, "source with spaces.svg"), filepath.Join(directory, "icon.svg")
	input := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 23.99999 24"><g id="layer"><path style="fill:#000000;stroke:black;stroke-width:11.5295" d="M .999 .89 L 1.23456 2.34567"/></g></svg>`
	if err := os.WriteFile(source, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	args := []string{"--icon", "--precision", "2", "--selective-rounding", "--current-color", "--output", destination, source}
	if err := RunSVGOptimizationArgs(nil, args, &output); err != nil {
		t.Fatal(err)
	}
	var receipt SVGOptimizationReceipt
	if err := json.Unmarshal(output.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	result := receipt.Files[0]
	prepared, err := os.ReadFile(destination)
	if err != nil || !bytes.Contains(prepared, []byte(`d="M1 .9L1.23 2.35"`)) || !bytes.Contains(prepared, []byte(`fill="currentColor"`)) {
		t.Fatalf("%s, %v", prepared, err)
	}
	if !bytes.Contains(prepared, []byte(`viewBox="0 0 24 24"`)) || !bytes.Contains(prepared, []byte(`stroke-width="11.53"`)) {
		t.Fatalf("presentation precision: %s", prepared)
	}
	if result.Path != source || result.OutputPath != destination || !result.Changed || result.InputBytes != len(input) || result.OutputBytes != len(prepared) || len(result.PathErrors) != 1 {
		t.Fatalf("%+v", result)
	}
	original, _ := os.ReadFile(source)
	if string(original) != input {
		t.Fatal("changed source")
	}
	output.Reset()
	if err := RunSVGOptimizationArgs(nil, args, &output); err == nil {
		t.Fatal("overwrote destination")
	}
	if err := json.Unmarshal(output.Bytes(), &receipt); err != nil || receipt.Files[0].Error == "" {
		t.Fatalf("missing file failure: %s", output.Bytes())
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
