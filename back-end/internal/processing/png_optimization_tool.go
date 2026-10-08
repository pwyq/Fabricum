package processing

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runPNGOptimizer(ctx context.Context, input []byte, directory string) ([]byte, string, error) {
	executable, err := findNativeTool("oxipng", "PNG optimization", directory)
	if err != nil {
		return nil, "", err
	}
	versionCommand := exec.CommandContext(ctx, executable, "--version")
	versionCommand.Env = nativeToolEnvironment(executable)
	versionBytes, err := versionCommand.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return nil, "", fmt.Errorf("PNG optimization canceled: %w", ctx.Err())
		}
		return nil, "", fmt.Errorf("query oxipng version: %w: %s", err, strings.TrimSpace(string(versionBytes)))
	}
	version := strings.TrimPrefix(strings.TrimSpace(string(versionBytes)), "oxipng ")
	if version != OxipngVersion {
		return nil, version, fmt.Errorf("PNG optimization requires oxipng %s, got %q", OxipngVersion, version)
	}
	workspace, err := os.MkdirTemp("", "fabricum-oxipng-")
	if err != nil {
		return nil, version, fmt.Errorf("create PNG optimization workspace: %w", err)
	}
	defer os.RemoveAll(workspace)
	inputPath, outputPath := filepath.Join(workspace, "input.png"), filepath.Join(workspace, "output.png")
	if err := os.WriteFile(inputPath, input, 0600); err != nil {
		return nil, version, fmt.Errorf("stage PNG: %w", err)
	}
	args := []string{"-o", "6", "--nx", "--interlace", "keep", "--threads", "1", "--force", "--out", outputPath, inputPath}
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = workspace
	command.Env = nativeToolEnvironment(executable)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, version, fmt.Errorf("PNG optimization canceled: %w", ctx.Err())
		}
		return nil, version, nativeToolRunError("PNG optimization", "oxipng", executable, err, stderr.String())
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, version, fmt.Errorf("read oxipng output: %w", err)
	}
	return data, version, nil
}
