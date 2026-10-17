package compatibility

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"

	fabricum "fabricum/back-end"
)

func runJPEGWorkflow(ctx context.Context, fixture fixtures) (fabricum.JPEGOptimizationReceipt, error) {
	path := filepath.Join(fixture.Root, "optimize.jpg")
	source := patternedImage(67, 49, func(x, y int) color.NRGBA {
		return color.NRGBA{R: uint8(x * 3), G: uint8(y * 5), B: 80, A: 255}
	})
	var input bytes.Buffer
	var receipt fabricum.JPEGOptimizationReceipt
	if err := jpeg.Encode(&input, source, &jpeg.Options{Quality: 92}); err != nil {
		return receipt, err
	}
	if err := os.WriteFile(path, input.Bytes(), 0600); err != nil {
		return receipt, err
	}
	var output bytes.Buffer
	if err := fabricum.OptimizeJPEGArgs(ctx, []string{"--progressive", path}, &output); err != nil {
		return receipt, fmt.Errorf("JPEG optimization: %w", err)
	}
	if err := readJSON(output.Bytes(), &receipt); err != nil {
		return receipt, err
	}
	if receipt.SchemaVersion != fabricum.JPEGOptimizationReceiptSchemaVersion || len(receipt.Files) != 1 || receipt.Processor == "" {
		return receipt, fmt.Errorf("incomplete JPEG optimization receipt")
	}
	result := receipt.Files[0]
	if !result.Changed || !result.Progressive || result.SavingsBytes <= 0 || result.OutputBytes >= result.InputBytes || result.OptimizerVersion != fabricum.JPEGTranVersion {
		return receipt, fmt.Errorf("JPEG optimizer did not reduce fixture: %+v", result)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return receipt, err
	}
	before, err := jpeg.Decode(bytes.NewReader(input.Bytes()))
	if err != nil {
		return receipt, err
	}
	after, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return receipt, err
	}
	if before.Bounds() != after.Bounds() {
		return receipt, fmt.Errorf("JPEG optimization changed dimensions")
	}
	for y := 0; y < 49; y++ {
		for x := 0; x < 67; x++ {
			if color.NRGBAModel.Convert(before.At(x, y)) != color.NRGBAModel.Convert(after.At(x, y)) {
				return receipt, fmt.Errorf("JPEG optimization changed samples at (%d,%d)", x, y)
			}
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return receipt, err
	}
	again, err := fabricum.OptimizeJPEG(ctx, fabricum.JPEGOptimizationRequest{Path: path})
	if err != nil || again.Changed || again.OutputBytes != result.OutputBytes {
		return receipt, fmt.Errorf("repeat JPEG optimization changed file: %+v, %v", again, err)
	}
	repeated, err := os.ReadFile(path)
	if err != nil {
		return receipt, err
	}
	current, err := os.Stat(path)
	if err != nil || !bytes.Equal(data, repeated) || !info.ModTime().Equal(current.ModTime()) {
		return receipt, fmt.Errorf("repeat JPEG optimization rewrote bytes or timestamp")
	}
	return receipt, nil
}
