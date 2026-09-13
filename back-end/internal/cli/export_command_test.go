package cli

import (
	"path/filepath"
	"testing"

	"fabricum/back-end/internal/processing"
)

func TestExportCommandReturnsCommandFailure(t *testing.T) {
	directory := t.TempDir()
	callback := exportCommand([]string{"node", "-e", "process.exit(7)"}, directory)
	err := callback(filepath.Join(directory, "source.png"), processing.ExportRequest{}, nil)
	if err == nil {
		t.Fatal("hook failure must be returned")
	}
}
