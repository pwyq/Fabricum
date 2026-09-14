package compatibility

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	fabricum "fabricum/back-end"
)

const ReceiptSchemaVersion = 1

// Receipt is the node-free end-to-end compatibility report. The nested
// receipts are the same versioned contracts emitted by each Fabricum command.
type Receipt struct {
	SchemaVersion int                       `json:"schemaVersion"`
	Processor     string                    `json:"processor"`
	FixturePolicy string                    `json:"fixturePolicy"`
	ImageExports  []fabricum.ExportReceipt  `json:"imageExports"`
	Sprites       fabricum.TransformReceipt `json:"sprites"`
	MaterialSet   fabricum.TextureReceipt   `json:"materialSet"`
	Models        []fabricum.ModelReceipt   `json:"models"`
	Inspection    fabricum.InspectionReport `json:"inspection"`
}

func writeJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", filepath.Base(path), err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

func readJSON(data []byte, value any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("decode receipt: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return fmt.Errorf("receipt contains trailing JSON")
	}
	return nil
}

func checkBytes(path string, expectedBytes int, expectedHash string) error {
	data, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		return fmt.Errorf("read receipt output %s: %w", path, err)
	}
	hash := sha256.Sum256(data)
	actualHash := hex.EncodeToString(hash[:])
	if len(data) != expectedBytes || actualHash != expectedHash {
		return fmt.Errorf("receipt output %s has bytes/hash %d/%s, want %d/%s", path, len(data), actualHash, expectedBytes, expectedHash)
	}
	return nil
}

func checkImageMeasurement(output fabricum.OutputMeasurement) error {
	if output.Path == "" || output.Format == "" || output.Encoder == "" || output.Processor == "" || output.Bytes <= 0 || len(output.SHA256) != 64 {
		return fmt.Errorf("incomplete image receipt for %s: %+v", output.Role, output)
	}
	if err := checkBytes(output.Path, output.Bytes, output.SHA256); err != nil {
		return err
	}
	if output.Format != "png" && (output.EncoderVersion == "" || len(output.NativeEncoderVersions) == 0) {
		return fmt.Errorf("missing native provenance for %s: %+v", output.Role, output)
	}
	return nil
}

func checkModelMeasurement(output fabricum.ModelOutputMeasurement) error {
	if output.Path == "" || output.Format == "" || output.Tool == "" || output.ToolVersion == "" || output.Bytes <= 0 || len(output.SHA256) != 64 {
		return fmt.Errorf("incomplete model receipt for %s: %+v", output.Role, output)
	}
	return checkBytes(output.Path, output.Bytes, output.SHA256)
}
