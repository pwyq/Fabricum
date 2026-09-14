package compatibility

import (
	"context"
	"fmt"
	"path/filepath"

	fabricum "fabricum/back-end"
)

func runTextureWorkflow(ctx context.Context, fixture fixtures) (fabricum.TextureReceipt, error) {
	request := fabricum.TextureSetRequest{
		MaxWorkers: 1, RequiredRoles: []string{"base-color", "normal", "arm"},
		Outputs: []fabricum.TextureOutputSpec{
			{Role: "base-color", Source: fixture.Source, Path: filepath.Join(fixture.Root, "material", "base-color.ktx2"), Encoding: fabricum.TextureEncodingOptions{
				Encoding: fabricum.TextureEncodingETC1S, MipLevels: 11, TransferFunction: fabricum.TextureTransferSRGB, ColorPrimaries: fabricum.TexturePrimariesBT709,
			}},
			{Role: "normal", Source: fixture.Normal, Path: filepath.Join(fixture.Root, "material", "normal.ktx2"), Encoding: fabricum.TextureEncodingOptions{
				Encoding: fabricum.TextureEncodingUASTCZstd, MipLevels: 11, TransferFunction: fabricum.TextureTransferLinear, ColorPrimaries: fabricum.TexturePrimariesUnspecified, ZstdLevel: 6,
			}},
			{Role: "arm", Source: fixture.Source, Path: filepath.Join(fixture.Root, "material", "arm.ktx2"), Transform: fabricum.ImageTransform{Pack: &fabricum.ChannelPack{
				Red: fabricum.ChannelInput{Source: fixture.AO, Channel: "red"}, Green: fabricum.ChannelInput{Source: fixture.Roughness, Channel: "red"}, Blue: fabricum.ChannelInput{Constant: bytePointer(0)},
			}}, Encoding: fabricum.TextureEncodingOptions{
				Encoding: fabricum.TextureEncodingUASTCZstd, MipLevels: 11, TransferFunction: fabricum.TextureTransferLinear, ColorPrimaries: fabricum.TexturePrimariesUnspecified, ZstdLevel: 6,
			}},
		},
	}
	requestPath := filepath.Join(fixture.Root, "material.json")
	if err := writeJSON(requestPath, request); err != nil {
		return fabricum.TextureReceipt{}, err
	}
	var receipt fabricum.TextureReceipt
	if err := runReceiptFile(ctx, requestPath, fabricum.TextureSetFile, &receipt); err != nil {
		return fabricum.TextureReceipt{}, fmt.Errorf("material set: %w", err)
	}
	if receipt.SchemaVersion != fabricum.TextureReceiptSchemaVersion || receipt.Processor == "" || len(receipt.Outputs) != 3 {
		return fabricum.TextureReceipt{}, fmt.Errorf("material set returned an incomplete receipt: %+v", receipt)
	}
	expected := map[string]struct {
		encoding, transfer, primaries string
	}{
		"base-color": {fabricum.TextureEncodingETC1S, fabricum.TextureTransferSRGB, fabricum.TexturePrimariesBT709},
		"normal":     {fabricum.TextureEncodingUASTCZstd, fabricum.TextureTransferLinear, fabricum.TexturePrimariesUnspecified},
		"arm":        {fabricum.TextureEncodingUASTCZstd, fabricum.TextureTransferLinear, fabricum.TexturePrimariesUnspecified},
	}
	for _, output := range receipt.Outputs {
		want, ok := expected[output.Role]
		if !ok || output.Width != 1024 || output.Height != 1024 || output.MipLevels != 11 || output.Encoding != want.encoding || output.TransferFunction != want.transfer || output.ColorPrimaries != want.primaries {
			return fabricum.TextureReceipt{}, fmt.Errorf("material output does not match contract: %+v", output)
		}
		if err := checkImageMeasurement(output); err != nil {
			return fabricum.TextureReceipt{}, err
		}
	}
	if receipt.NativeEncoderVersions["basisu"] == "" {
		return fabricum.TextureReceipt{}, fmt.Errorf("material receipt is missing Basis Universal identity")
	}
	return receipt, nil
}

func bytePointer(value byte) *uint8 {
	converted := uint8(value)
	return &converted
}
