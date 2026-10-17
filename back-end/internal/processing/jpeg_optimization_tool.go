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

func runJPEGOptimizer(ctx context.Context, input []byte, directory string, progressive bool) ([]byte, string, error) {
	executable, err := findNativeTool("jpegtran", "JPEG optimization", directory)
	if err != nil {
		return nil, "", err
	}
	versionCommand := exec.CommandContext(ctx, executable, "-version")
	versionCommand.Env = nativeToolEnvironment(executable)
	versionBytes, err := versionCommand.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return nil, "", fmt.Errorf("JPEG optimization canceled: %w", ctx.Err())
		}
		return nil, "", fmt.Errorf("query jpegtran version: %w: %s", err, strings.TrimSpace(string(versionBytes)))
	}
	fields := strings.Fields(string(versionBytes))
	version := strings.TrimSpace(string(versionBytes))
	if len(fields) >= 3 && fields[0] == "libjpeg-turbo" && fields[1] == "version" {
		version = fields[2]
	}
	if version != JPEGTranVersion {
		return nil, version, fmt.Errorf("JPEG optimization requires libjpeg-turbo jpegtran %s, got %q", JPEGTranVersion, version)
	}
	workspace, err := os.MkdirTemp("", "fabricum-jpegtran-")
	if err != nil {
		return nil, version, err
	}
	defer os.RemoveAll(workspace)
	inputPath := filepath.Join(workspace, "input.jpg")
	if err := os.WriteFile(inputPath, input, 0600); err != nil {
		return nil, version, fmt.Errorf("stage JPEG: %w", err)
	}
	candidate, err := transcodeJPEG(ctx, executable, workspace, inputPath, "candidate.jpg", progressive)
	if err != nil {
		return nil, version, err
	}
	source, err := parseOptimizationJPEG(input)
	if err != nil {
		return nil, version, err
	}
	optimized, err := parseOptimizationJPEG(candidate)
	if err != nil {
		return nil, version, fmt.Errorf("invalid jpegtran output: %w", err)
	}
	candidate = rebuildOptimizedJPEG(source, optimized)
	if err := os.WriteFile(filepath.Join(workspace, "candidate.jpg"), candidate, 0600); err != nil {
		return nil, version, err
	}
	// Serialize both coefficient arrays using identical sequential Huffman
	// coding. Exact equality checks coefficients, quantization, component IDs,
	// sampling and dimensions, even changes hidden by rounded decoded pixels.
	before, err := transcodeJPEG(ctx, executable, workspace, inputPath, "before.jpg", false)
	if err != nil {
		return nil, version, err
	}
	after, err := transcodeJPEG(ctx, executable, workspace, filepath.Join(workspace, "candidate.jpg"), "after.jpg", false)
	if err != nil {
		return nil, version, err
	}
	if !bytes.Equal(before, after) {
		return nil, version, fmt.Errorf("jpegtran violated the lossless DCT coefficient contract")
	}
	return candidate, version, nil
}

func transcodeJPEG(ctx context.Context, executable, workspace, inputPath, outputName string, progressive bool) ([]byte, error) {
	outputPath := filepath.Join(workspace, outputName)
	args := []string{"-strict", "-copy", "none", "-optimize", "-maxscans", "256", "-maxmemory", "512M"}
	if progressive {
		args = append(args, "-progressive")
	}
	args = append(args, "-outfile", outputPath, inputPath)
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir, command.Env = workspace, nativeToolEnvironment(executable)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("JPEG optimization canceled: %w", ctx.Err())
		}
		return nil, nativeToolRunError("JPEG optimization", "jpegtran", executable, err, stderr.String())
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read jpegtran output: %w", err)
	}
	if info.Size() > maxInspectionFileBytes {
		return nil, fmt.Errorf("jpegtran output exceeds file size limit")
	}
	return os.ReadFile(outputPath)
}
