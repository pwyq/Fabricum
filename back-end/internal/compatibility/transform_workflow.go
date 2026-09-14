package compatibility

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"

	fabricum "fabricum/back-end"
)

func runSpriteWorkflow(ctx context.Context, fixture fixtures) (fabricum.TransformReceipt, error) {
	request := fabricum.TransformRequest{
		Source:      fixture.Transparent,
		Constraints: fabricum.SourceConstraints{Format: "png", SingleFrame: true, Width: 160, Height: 80},
		Format:      "png",
		Outputs: []fabricum.TransformOutputSpec{
			{Role: "sprite-256", Path: filepath.Join(fixture.Root, "sprites", "sprite-256.png"), Transform: fabricum.ImageTransform{
				Resize: &fabricum.ResizeSpec{Width: 256, Height: 256, Fit: "contain", Filter: "lanczos3"},
			}},
			{Role: "sprite-512", Path: filepath.Join(fixture.Root, "sprites", "sprite-512.png"), Transform: fabricum.ImageTransform{
				Resize: &fabricum.ResizeSpec{Width: 512, Height: 512, Fit: "contain", Filter: "lanczos3"},
			}},
		},
	}
	requestPath := filepath.Join(fixture.Root, "sprites.json")
	if err := writeJSON(requestPath, request); err != nil {
		return fabricum.TransformReceipt{}, err
	}
	var receipt fabricum.TransformReceipt
	if err := runReceiptFile(ctx, requestPath, fabricum.TransformFile, &receipt); err != nil {
		return fabricum.TransformReceipt{}, fmt.Errorf("sprites: %w", err)
	}
	if receipt.SchemaVersion != fabricum.TransformReceiptSchemaVersion || len(receipt.Outputs) != 2 || receipt.Processor == "" {
		return fabricum.TransformReceipt{}, fmt.Errorf("sprites returned an incomplete receipt: %+v", receipt)
	}
	for _, output := range receipt.Outputs {
		if err := checkImageMeasurement(output); err != nil {
			return fabricum.TransformReceipt{}, err
		}
		data, err := os.ReadFile(filepath.FromSlash(output.Path))
		if err != nil {
			return fabricum.TransformReceipt{}, err
		}
		decoded, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return fabricum.TransformReceipt{}, fmt.Errorf("decode %s: %w", output.Role, err)
		}
		if decoded.Bounds().Dx() != output.Width || decoded.Bounds().Dy() != output.Height {
			return fabricum.TransformReceipt{}, fmt.Errorf("%s dimensions do not match receipt", output.Role)
		}
		if alphaAt(decoded, 0, 0) != 0 || alphaAt(decoded, output.Width/2, output.Height/2) == 0 {
			return fabricum.TransformReceipt{}, fmt.Errorf("%s does not contain transparent padding around an opaque sprite", output.Role)
		}
	}
	return receipt, nil
}

func runReceiptFile[T any](ctx context.Context, path string, run func(context.Context, string, io.Writer) error, receipt *T) error {
	var output bytes.Buffer
	if err := run(ctx, path, &output); err != nil {
		return err
	}
	return readJSON(output.Bytes(), receipt)
}

func alphaAt(source interface{ At(int, int) color.Color }, x, y int) uint32 {
	_, _, _, alpha := source.At(x, y).RGBA()
	return alpha
}
