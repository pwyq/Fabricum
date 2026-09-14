package processing

import (
	"bytes"
	"context"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTextureContractUsesFullMipsAndBoundedWorkers(t *testing.T) {
	if got := generatedMipLevels(1024, 1024); got != 11 {
		t.Fatalf("generated mip count = %d, want 11", got)
	}
	options := TextureEncodingOptions{Encoding: TextureEncodingUASTCZstd, MipLevels: 11, TransferFunction: TextureTransferLinear, ColorPrimaries: TexturePrimariesUnspecified, ZstdLevel: 6}
	normalized, err := normalizeTextureEncoding(options, 1024, 1024)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(basisUArgs(normalized, "input.png", "output.ktx2"), " ")
	for _, expected := range []string{"-uastc", "-ktx2_zstandard_level 6", "-mipmap", "-no_multithreading", "-linear"} {
		if !strings.Contains(args, expected) {
			t.Fatalf("basisu args %q do not contain %q", args, expected)
		}
	}
	if workers, err := textureWorkerCount(1, 3); err != nil || workers != 1 {
		t.Fatalf("one-worker request = %d, %v", workers, err)
	}
	if _, err := textureWorkerCount(maxTextureWorkers+1, 3); err == nil {
		t.Fatal("accepted an unbounded worker request")
	}
	data := syntheticKTX2(1024, 1024, 11)
	if err := patchKTX2Metadata(data, normalized); err != nil {
		t.Fatal(err)
	}
	facts, err := inspectKTX2(data)
	if err != nil {
		t.Fatal(err)
	}
	if facts.TransferFunction != TextureTransferLinear || facts.ColorPrimaries != TexturePrimariesUnspecified {
		t.Fatalf("patched KTX2 metadata = %+v", facts)
	}
}

func TestTextureValidationRejectsInvalidColorContracts(t *testing.T) {
	cases := []TextureEncodingOptions{
		{Encoding: TextureEncodingETC1S, TransferFunction: "pq"},
		{Encoding: TextureEncodingETC1S, ColorPrimaries: "display-p3"},
		{Encoding: TextureEncodingETC1S, ZstdLevel: 1},
		{Encoding: TextureEncodingUASTCZstd, MipLevels: 2},
	}
	for _, options := range cases {
		if _, err := normalizeTextureEncoding(options, 4, 4); err == nil {
			t.Fatalf("accepted invalid texture contract: %+v", options)
		}
	}
}

func TestTextureSetRejectsIncompleteMaterialBeforeWriting(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "base.ktx2")
	original := []byte("existing delivery")
	if err := os.WriteFile(existing, original, 0600); err != nil {
		t.Fatal(err)
	}
	request := TextureSetRequest{
		RequiredRoles: []string{"base-color", "normal", "arm"},
		Outputs:       []TextureOutputSpec{{Role: "base-color", Source: filepath.Join(root, "missing.png"), Path: existing}},
	}
	if _, err := BuildTextureSet(context.Background(), request); err == nil {
		t.Fatal("accepted an incomplete material set")
	}
	data, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, original) {
		t.Fatal("incomplete material set replaced an existing delivery")
	}
}

func TestSyntheticMaterialSetUsesBothKTX2Contracts(t *testing.T) {
	encoderDirectory := availableBasisDirectory(t)
	root := t.TempDir()
	base := filepath.Join(root, "base.png")
	normal := filepath.Join(root, "normal.png")
	ao := filepath.Join(root, "ao.png")
	roughness := filepath.Join(root, "roughness.png")
	writeTransformFixture(t, base, 4, 4, func(x, y int) color.NRGBA {
		return color.NRGBA{R: uint8(20 + x), G: uint8(30 + y), B: 80, A: 255}
	})
	writeTransformFixture(t, normal, 4, 4, func(x, y int) color.NRGBA {
		return color.NRGBA{R: uint8(120 + x), G: uint8(125 + y), B: 255, A: 255}
	})
	writeTransformFixture(t, ao, 4, 4, func(x, y int) color.NRGBA {
		return color.NRGBA{R: uint8(10 + x + y), A: 255}
	})
	writeTransformFixture(t, roughness, 4, 4, func(x, y int) color.NRGBA {
		return color.NRGBA{R: uint8(90 + x + y), A: 255}
	})
	request := TextureSetRequest{MaxWorkers: 1, EncoderDirectory: encoderDirectory, RequiredRoles: []string{"base-color", "normal", "arm"}, Outputs: []TextureOutputSpec{
		{Role: "base-color", Source: base, Path: filepath.Join(root, "base.ktx2"), Encoding: TextureEncodingOptions{Encoding: TextureEncodingETC1S, MipLevels: 3, TransferFunction: TextureTransferSRGB, ColorPrimaries: TexturePrimariesBT709}},
		{Role: "normal", Source: normal, Path: filepath.Join(root, "normal.ktx2"), Encoding: TextureEncodingOptions{Encoding: TextureEncodingUASTCZstd, MipLevels: 3, TransferFunction: TextureTransferLinear, ColorPrimaries: TexturePrimariesUnspecified, ZstdLevel: 6}},
		{Role: "arm", Source: base, Path: filepath.Join(root, "arm.ktx2"), Transform: ImageTransform{Pack: &ChannelPack{Red: ChannelInput{Source: ao, Channel: "red"}, Green: ChannelInput{Source: roughness, Channel: "red"}, Blue: ChannelInput{Constant: uint8Pointer(0)}}}, Encoding: TextureEncodingOptions{Encoding: TextureEncodingUASTCZstd, MipLevels: 3, TransferFunction: TextureTransferLinear, ColorPrimaries: TexturePrimariesUnspecified, ZstdLevel: 6}},
	}}
	first, err := PrepareTextureSetOutputs(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteOutputs(first); err != nil {
		t.Fatal(err)
	}
	second, err := PrepareTextureSetOutputs(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	for index, output := range first {
		if !bytes.Equal(output.Data, second[index].Data) || output.Measurement.Encoding == "" || output.Measurement.MipLevels != 3 || output.Measurement.Bytes != len(output.Data) || len(output.Measurement.SHA256) != 64 {
			t.Fatalf("output %d is missing stable KTX2 receipt data: %+v", index, output.Measurement)
		}
	}
}

func availableBasisDirectory(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("basisu"); err == nil {
		return ""
	}
	directory := filepath.Join("..", "..", "..", "bin", "codecs")
	name := "basisu"
	if filepath.Separator == '\\' {
		name += ".exe"
	}
	if _, err := os.Stat(filepath.Join(directory, name)); err != nil {
		t.Skip("pinned basisu is not installed")
	}
	return directory
}
