package cli

import (
	"encoding/json"
	"errors"
	"fabricum/back-end/internal/editor"
	"fabricum/back-end/internal/processing"
	"fmt"
	"io"
)

// RunInspection writes one versioned JSON report for the requested paths.
// File failures are written in the report and returned after the report is
// complete so callers can consume all results while still seeing failure.
func RunInspection(paths []string, output io.Writer) error {
	results, err := processing.InspectFiles(paths)
	if err != nil {
		return fmt.Errorf("inspect: %w", err)
	}
	report := processing.InspectionReport{
		SchemaVersion: processing.InspectionSchemaVersion,
		Processor:     "fabricum/" + editor.Version,
		Assets:        results,
	}
	if err := json.NewEncoder(output).Encode(report); err != nil {
		return fmt.Errorf("write inspection report: %w", err)
	}
	if processing.HasInspectionErrors(results) {
		return errors.New("inspect: one or more assets could not be inspected")
	}
	return nil
}
