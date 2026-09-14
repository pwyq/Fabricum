package processing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

func findGltfpack(request ModelOptimizationRequest) (string, error) {
	directory := request.GltfpackDirectory
	if directory == "" {
		directory = request.EncoderDirectory
	}
	return findNativeTool("gltfpack", "model", directory)
}

func runGltfpack(ctx context.Context, executable string, request ModelOptimizationRequest, inputPath, outputPath, directory string) ([]byte, error) {
	command := exec.CommandContext(ctx, executable, modelGltfpackArgs(request, inputPath, outputPath)...)
	command.Dir = directory
	command.Env = nativeToolEnvironment(executable)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("gltfpack optimization canceled: %w", ctxErr)
		}
		return nil, nativeToolStartError("model", "gltfpack", executable, err)
	}
	if err := command.Wait(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("gltfpack optimization canceled: %w", ctxErr)
		}
		return nil, nativeToolRunError("model", "gltfpack", executable, err, stderr.String())
	}
	return stderr.Bytes(), nil
}

func modelGltfpackArgs(request ModelOptimizationRequest, inputPath, outputPath string) []string {
	args := []string{"-i", inputPath, "-o", outputPath, "-km"}
	if request.Compression == ModelCompressionMeshopt {
		args = append(args, "-c")
	} else {
		args = append(args, "-noq")
	}
	switch request.TextureCompression {
	case ModelTextureKTX2:
		args = append(args, "-tc")
		if request.TextureEncoding == ModelTextureUASTC {
			args = append(args, "-tu")
		}
		args = append(args, "-tq", fmt.Sprintf("%d", request.TextureQuality))
	case ModelTextureWebP:
		args = append(args, "-tw", "-tq", fmt.Sprintf("%d", request.TextureQuality))
	}
	return args
}

func writeModelJSON(path string, object map[string]json.RawMessage) error {
	data, err := json.Marshal(object)
	if err != nil {
		return fmt.Errorf("encode staged glTF: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		return fmt.Errorf("write staged glTF: %w", err)
	}
	return nil
}
