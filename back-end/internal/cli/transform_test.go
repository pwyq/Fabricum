package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunTransformFileWritesReceiptAndResolvesRelativePaths(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.png")
	writeFixtureImage(t, source, 8, 8)
	requestPath := filepath.Join(root, "transform.json")
	request := map[string]any{
		"source":      "source.png",
		"constraints": map[string]any{"format": "png", "width": 8, "height": 8},
		"format":      "png",
		"outputs": []any{map[string]any{
			"role":      "sprite",
			"path":      "delivery/sprite.png",
			"transform": map[string]any{"resize": map[string]any{"width": 4, "height": 4, "fit": "contain", "filter": "lanczos3"}},
		}},
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(requestPath, data, 0600); err != nil {
		t.Fatal(err)
	}

	var first bytes.Buffer
	if err := RunTransformFile(context.Background(), requestPath, &first); err != nil {
		t.Fatal(err)
	}
	var receipt TransformReceipt
	if err := json.Unmarshal(first.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.SchemaVersion != TransformReceiptSchemaVersion || len(receipt.Outputs) != 1 || receipt.Source != source {
		t.Fatalf("unexpected transform receipt: %+v", receipt)
	}
	if receipt.Outputs[0].Bytes <= 0 || len(receipt.Outputs[0].SHA256) != 64 || receipt.Outputs[0].Filter != "lanczos3" {
		t.Fatalf("receipt is missing output provenance: %+v", receipt.Outputs[0])
	}
	outputPath := filepath.Join(root, "delivery", "sprite.png")
	assertImageDimensions(t, outputPath, 4, 4)

	var second bytes.Buffer
	if err := RunTransformFile(context.Background(), requestPath, &second); err != nil {
		t.Fatal(err)
	}
	var repeated TransformReceipt
	if err := json.Unmarshal(second.Bytes(), &repeated); err != nil {
		t.Fatal(err)
	}
	if receipt.Outputs[0].SHA256 != repeated.Outputs[0].SHA256 {
		t.Fatalf("repeated receipt changed SHA-256: %q != %q", receipt.Outputs[0].SHA256, repeated.Outputs[0].SHA256)
	}
}
