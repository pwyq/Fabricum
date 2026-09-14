package cli

import (
	"bytes"
	"encoding/json"
	"fabricum/back-end/internal/editor"
	"fabricum/back-end/internal/processing"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInspectionWritesVersionedReportAndFailsAfterPerFileResults(t *testing.T) {
	root := t.TempDir()
	valid := filepath.Join(root, "valid.png")
	writeFixtureImage(t, valid, 4, 3)
	missing := filepath.Join(root, "missing.png")
	var output bytes.Buffer
	err := RunInspection([]string{valid, missing}, &output)
	if err == nil {
		t.Fatal("inspection with a missing file should fail")
	}
	var report processing.InspectionReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("invalid inspection report: %v\n%s", err, output.String())
	}
	if report.SchemaVersion != processing.InspectionSchemaVersion || report.Processor != "fabricum/"+editor.Version || len(report.Assets) != 2 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Assets[0].Path != valid || report.Assets[0].Error != "" || report.Assets[1].Path != missing || report.Assets[1].Error == "" {
		t.Fatalf("unexpected ordered results: %+v", report.Assets)
	}
	if strings.Contains(strings.ToLower(output.String()), "sha256") {
		t.Fatal("inspection report must not contain content hashes")
	}
}
