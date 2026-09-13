package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fabricum/back-end/internal/editor"
	"fabricum/back-end/internal/processing"
	"fmt"
	"os/exec"
	"time"
)

const ExportReceiptSchemaVersion = 1

// ExportReceipt is sent to a configured command after delivery files are written.
// Schema version 1 is additive: integrations must ignore unknown properties.
type ExportReceipt struct {
	SchemaVersion int                            `json:"schemaVersion"`
	Processor     string                         `json:"processor"`
	Source        string                         `json:"source"`
	Request       processing.ExportRequest       `json:"request"`
	Outputs       []processing.OutputMeasurement `json:"outputs"`
}

func exportCommand(argv []string, directory string) func(string, processing.ExportRequest, []processing.OutputMeasurement) error {
	return func(source string, request processing.ExportRequest, outputs []processing.OutputMeasurement) error {
		data, err := json.Marshal(ExportReceipt{
			SchemaVersion: ExportReceiptSchemaVersion,
			Processor:     "fabricum/" + editor.Version,
			Source:        source,
			Request:       request,
			Outputs:       outputs,
		})
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
