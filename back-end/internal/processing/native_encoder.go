package processing

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func encodeNative(ctx context.Context, pngInput []byte, request ExportRequest, encoderDirectory string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	executableName, args := nativeEncoderCommand(request)
	executable, err := findNativeEncoder(executableName, request.Format, encoderDirectory)
	if err != nil {
		return nil, err
	}
	temporary, err := os.MkdirTemp("", "fabricum-codec-")
	if err != nil {
		return nil, fmt.Errorf("create native codec workspace: %w", err)
	}
	defer os.RemoveAll(temporary)
	inputPath := filepath.Join(temporary, "input.png")
	outputPath := filepath.Join(temporary, request.Format)
	if err := os.WriteFile(inputPath, pngInput, 0o600); err != nil {
		return nil, fmt.Errorf("stage PNG for %s encoder: %w", request.Format, err)
	}
	if request.Format == "webp" {
		args = append(args, outputPath, inputPath)
	} else {
		args = append(args, inputPath, outputPath)
	}
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = temporary
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("%s encoding canceled: %w", request.Format, ctxErr)
		}
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return nil, fmt.Errorf("%s encoder failed: %w: %s", request.Format, err, message)
		}
		return nil, fmt.Errorf("%s encoder failed: %w", request.Format, err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read %s encoder output: %w", request.Format, err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%s encoder produced an empty output", request.Format)
	}
	if !validNativeOutput(request.Format, data) {
		return nil, fmt.Errorf("%s encoder produced invalid output", request.Format)
	}
	return data, nil
}

func validNativeOutput(format string, data []byte) bool {
	switch format {
	case "webp":
		return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
	case "avif":
		return len(data) >= 12 && string(data[4:8]) == "ftyp"
	case "ktx2":
		return isKTX2(data)
	default:
		return false
	}
}

func nativeEncoderCommand(request ExportRequest) (string, []string) {
	if request.Format == "webp" {
		args := []string{"-quiet", "-preset", "picture", "-m", "6", "-alpha_q", "100", "-exact"}
		if request.Lossless {
			args = append(args, "-lossless", "-q", strconv.Itoa(request.Quality))
		} else {
			args = append(args, "-q", strconv.Itoa(request.Quality))
		}
		return "cwebp", append(args, "-o")
	}
	args := []string{"--codec", "aom", "--jobs", "1", "--speed", "0", "--depth", "8", "--yuv", "444"}
	if request.Lossless {
		args = append(args, "--lossless")
	} else {
		args = append(args, "--qcolor", strconv.Itoa(request.Quality), "--qalpha", strconv.Itoa(request.Quality))
	}
	return "avifenc", args
}

func findNativeEncoder(name, format, directory string) (string, error) {
	return findNativeTool(name, format, directory)
}

func findNativeTool(name, format, directory string) (string, error) {
	if directory == "" {
		if bundled := bundledNativeDirectory(name); bundled != "" {
			path := nativeToolPath(bundled, name)
			if _, err := exec.LookPath(path); err == nil {
				return path, nil
			}
		}
		path, err := exec.LookPath(name)
		if err == nil {
			return path, nil
		}
		return "", missingNativeEncoderError(name, format, "PATH")
	}
	absoluteDirectory, err := filepath.Abs(directory)
	if err != nil {
		return "", fmt.Errorf("resolve native codec directory %q: %w", directory, err)
	}
	path := nativeToolPath(absoluteDirectory, name)
	if _, err := exec.LookPath(path); err != nil {
		return "", missingNativeEncoderError(name, format, absoluteDirectory)
	}
	return path, nil
}

func nativeToolPath(directory, name string) string {
	path := filepath.Join(directory, name)
	if runtime.GOOS == "windows" && filepath.Ext(path) == "" {
		path += ".exe"
	}
	return path
}

func bundledNativeDirectory(name string) string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	executableDirectory := filepath.Dir(executable)
	root := filepath.Dir(executableDirectory)
	candidates := []string{
		filepath.Join(executableDirectory, "codecs"),
		executableDirectory,
		filepath.Join(root, "codecs"),
		filepath.Join(root, "native"),
	}
	for _, candidate := range candidates {
		if _, err := exec.LookPath(nativeToolPath(candidate, name)); err == nil {
			return candidate
		}
	}
	return ""
}

func missingNativeEncoderError(name, format, location string) error {
	return fmt.Errorf("%s output requires native %s (%s); install the pinned native tools or set --encoder-directory/--gltfpack-directory", format, name, location)
}
