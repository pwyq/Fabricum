package fabricum

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// ExportReceipt is sent to a configured command after delivery files are written.
type ExportReceipt struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Processor     string              `json:"processor"`
	Source        string              `json:"source"`
	Request       ExportRequest       `json:"request"`
	Outputs       []OutputMeasurement `json:"outputs"`
}

func exportCommand(argv []string, directory string) func(string, ExportRequest, []OutputMeasurement) error {
	return func(source string, request ExportRequest, outputs []OutputMeasurement) error {
		data, err := json.Marshal(ExportReceipt{1, "fabricum/" + Version, source, request, outputs})
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, argv[0], argv[1:]...)
		command.Dir = directory
		command.Stdin = bytes.NewReader(data)
		var stderr bytes.Buffer
		command.Stderr = &stderr
		if err := command.Run(); err != nil {
			return fmt.Errorf("outputs written but export command failed: %w: %s", err, stderr.String())
		}
		return nil
	}
}
