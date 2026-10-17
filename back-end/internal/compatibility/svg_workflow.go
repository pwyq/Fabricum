package compatibility

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	fabricum "fabricum/back-end"
)

func runSVGWorkflow(ctx context.Context, fixture fixtures) (fabricum.SVGOptimizationReceipt, error) {
	path := filepath.Join(fixture.Root, "optimize.svg")
	input := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24">
  <!-- Generated vector-editor fixture; no external resources. -->
  <defs><linearGradient id="paint"><stop offset="0" stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient></defs>
  <g transform="translate(0)"><g><rect width="24" height="24" fill="url(#paint)"/></g></g>
</svg>`)
	var receipt fabricum.SVGOptimizationReceipt
	if err := os.WriteFile(path, input, 0600); err != nil {
		return receipt, err
	}
	var output bytes.Buffer
	if err := fabricum.OptimizeSVGArgs(ctx, []string{path}, &output); err != nil {
		return receipt, fmt.Errorf("SVG optimization: %w", err)
	}
	if err := readJSON(output.Bytes(), &receipt); err != nil {
		return receipt, err
	}
	if receipt.SchemaVersion != fabricum.SVGOptimizationReceiptSchemaVersion || receipt.Processor == "" || len(receipt.Files) != 1 {
		return receipt, fmt.Errorf("incomplete SVG optimization receipt")
	}
	result := receipt.Files[0]
	if !result.Changed || result.SavingsBytes <= 0 || result.OutputBytes >= result.InputBytes || result.Optimizer != "fabricum-svg" || result.OptimizerVersion != fabricum.SVGOptimizerVersion {
		return receipt, fmt.Errorf("SVG optimizer did not reduce fixture: %+v", result)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return receipt, err
	}
	if len(data) != result.OutputBytes || !bytes.Contains(data, []byte(`viewBox="0 0 24 24"`)) || !bytes.Contains(data, []byte(`fill="url(#paint)"`)) {
		return receipt, fmt.Errorf("SVG output lost its viewport or gradient reference")
	}
	info, err := os.Stat(path)
	if err != nil {
		return receipt, err
	}
	again, err := fabricum.OptimizeSVG(ctx, fabricum.SVGOptimizationRequest{Path: path})
	if err != nil || again.Changed || again.OutputBytes != result.OutputBytes {
		return receipt, fmt.Errorf("repeat SVG optimization changed file: %+v, %v", again, err)
	}
	repeated, err := os.ReadFile(path)
	if err != nil {
		return receipt, err
	}
	current, err := os.Stat(path)
	if err != nil || !bytes.Equal(data, repeated) || !info.ModTime().Equal(current.ModTime()) {
		return receipt, fmt.Errorf("repeat SVG optimization rewrote bytes or timestamp")
	}
	return receipt, nil
}
