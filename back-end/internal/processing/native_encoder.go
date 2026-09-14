package processing

import (
	"bytes"
	"context"
	"errors"
	"fabricum/back-end/internal/bundledtools"
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
	command.Env = nativeToolEnvironment(executable)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("%s encoding canceled: %w", request.Format, ctxErr)
		}
		return nil, nativeToolStartError(request.Format, executableName, executable, err)
	}
	if err := command.Wait(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("%s encoding canceled: %w", request.Format, ctxErr)
		}
		return nil, nativeToolRunError(request.Format, executableName, executable, err, stderr.String())
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
	if directory != "" {
		absoluteDirectory, err := filepath.Abs(directory)
		if err != nil {
			return "", fmt.Errorf("resolve native codec directory %q: %w", directory, err)
		}
		return findNativeToolInDirectory(name, format, absoluteDirectory)
	}
	if embedded, err := bundledtools.Tool(name); err == nil {
		return embedded, nil
	} else if !errors.Is(err, bundledtools.ErrNotBundled) {
		return "", fmt.Errorf("prepare embedded native %s for %s output: %w", name, format, err)
	}
	if bundled := bundledNativeDirectory(name); bundled != "" {
		return findNativeToolInDirectory(name, format, bundled)
	}
	path, err := exec.LookPath(name)
	if err == nil {
		return path, nil
	}
	return "", missingNativeEncoderError(name, format, "PATH")
}

func findNativeToolInDirectory(name, format, directory string) (string, error) {
	path := nativeToolPath(directory, name)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", missingNativeEncoderError(name, format, path)
		}
		return "", fmt.Errorf("inspect native %s for %s output at %s: %w", name, format, path, err)
	}
	if _, err := exec.LookPath(path); err != nil {
		return "", fmt.Errorf("native %s for %s output at %s is not executable: %w", name, format, path, err)
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

func nativeToolEnvironment(executable string) []string {
	if runtime.GOOS != "linux" {
		return nil
	}
	libraryPath := filepath.Dir(executable)
	if existing := os.Getenv("LD_LIBRARY_PATH"); existing != "" {
		libraryPath += string(os.PathListSeparator) + existing
	}
	environment := make([]string, 0, len(os.Environ())+1)
	for _, variable := range os.Environ() {
		if strings.HasPrefix(variable, "LD_LIBRARY_PATH=") {
			continue
		}
		environment = append(environment, variable)
	}
	return append(environment, "LD_LIBRARY_PATH="+libraryPath)
}

func bundledNativeDirectory(name string) string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	executableDirectory := filepath.Dir(executable)
	candidate := filepath.Join(executableDirectory, "codecs")
	if _, err := os.Stat(nativeToolPath(candidate, name)); err == nil {
		return candidate
	}
	return ""
}

func missingNativeEncoderError(name, format, location string) error {
	if location != "PATH" {
		return fmt.Errorf("%s output requires native %s at %s; use the matching standalone release or set --encoder-directory", format, name, location)
	}
	return fmt.Errorf("%s output requires native %s (%s); install the pinned native tools or set --encoder-directory/--gltfpack-directory", format, name, location)
}

func nativeToolRunError(format, name, executable string, err error, detail string) error {
	detail = strings.TrimSpace(detail)
	suffix := ""
	if detail != "" {
		suffix = ": " + detail
	}
	if looksLikeIncompatibleNativeTool(err) {
		return fmt.Errorf("%s output could not start native %s at %s; the bundled file or one of its runtime libraries is missing or incompatible: %w%s", format, name, executable, err, suffix)
	}
	return fmt.Errorf("%s output failed using native %s at %s: %w%s", format, name, executable, err, suffix)
}

func nativeToolStartError(format, name, executable string, err error) error {
	return fmt.Errorf("%s output could not start native %s at %s; the bundled file or one of its runtime libraries is missing or incompatible: %w", format, name, executable, err)
}

func looksLikeIncompatibleNativeTool(err error) bool {
	message := strings.ToLower(err.Error())
	for _, phrase := range []string{"exec format error", "bad exe format", "not a valid win32 application", "cannot execute binary file", "no such file or directory"} {
		if strings.Contains(message, phrase) {
			return true
		}
	}
	return false
}
