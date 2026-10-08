package compatibility

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	fabricum "fabricum/back-end"
)

func runPNGWorkflow(ctx context.Context, fixture fixtures) (fabricum.PNGOptimizationReceipt, error) {
	path := filepath.Join(fixture.Root, "optimize.png")
	source := patternedImage(64, 48, func(x, y int) color.NRGBA {
		return color.NRGBA{R: uint8(x * 3), G: uint8(y * 4), B: 80, A: uint8((x % 3) * 127)}
	})
	var input bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.NoCompression}).Encode(&input, source); err != nil {
		return fabricum.PNGOptimizationReceipt{}, err
	}
	if err := os.WriteFile(path, input.Bytes(), 0600); err != nil {
		return fabricum.PNGOptimizationReceipt{}, err
	}
	var output bytes.Buffer
	if err := fabricum.OptimizePNGArgs(ctx, []string{path}, &output); err != nil {
		return fabricum.PNGOptimizationReceipt{}, fmt.Errorf("PNG optimization: %w", err)
	}
	var receipt fabricum.PNGOptimizationReceipt
	if err := readJSON(output.Bytes(), &receipt); err != nil {
		return receipt, err
	}
	if receipt.SchemaVersion != fabricum.PNGOptimizationReceiptSchemaVersion || len(receipt.Files) != 1 || receipt.Processor == "" {
		return receipt, fmt.Errorf("incomplete PNG optimization receipt")
	}
	result := receipt.Files[0]
	if !result.Changed || result.SavingsBytes <= 0 || result.OutputBytes >= result.InputBytes || result.OptimizerVersion != fabricum.OxipngVersion {
		return receipt, fmt.Errorf("PNG optimizer did not reduce the fixture: %+v", result)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return receipt, err
	}
	pixels, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return receipt, err
	}
	if pixels.Bounds() != source.Bounds() {
		return receipt, fmt.Errorf("PNG optimization changed dimensions")
	}
	for y := 0; y < 48; y++ {
		for x := 0; x < 64; x++ {
			if color.NRGBAModel.Convert(source.At(x, y)) != color.NRGBAModel.Convert(pixels.At(x, y)) {
				return receipt, fmt.Errorf("PNG optimization changed RGBA at (%d,%d)", x, y)
			}
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return receipt, err
	}
	again, err := fabricum.OptimizePNG(ctx, fabricum.PNGOptimizationRequest{Path: path})
	if err != nil || again.Changed || again.OutputBytes != result.OutputBytes {
		return receipt, fmt.Errorf("repeat PNG optimization changed file: %+v, %v", again, err)
	}
	repeated, err := os.ReadFile(path)
	if err != nil {
		return receipt, err
	}
	current, err := os.Stat(path)
	if err != nil || !bytes.Equal(data, repeated) || !info.ModTime().Equal(current.ModTime()) {
		return receipt, fmt.Errorf("repeat PNG optimization rewrote bytes or timestamp")
	}
	return receipt, nil
}
