//go:build windows

package processing

import (
	"bytes"
	"context"
	"debug/pe"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestJPEGOptimizerWindowsRuntimeDependencies(t *testing.T) {
	directory := jpegTestToolDirectory(t)
	file, err := pe.Open(nativeToolPath(directory, "jpegtran"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	symbols, err := file.ImportedSymbols()
	if err != nil {
		t.Fatal(err)
	}
	for _, symbol := range symbols {
		parts := strings.Split(symbol, ":")
		library := parts[len(parts)-1]
		name := strings.ToLower(library)
		if name != "kernel32.dll" && name != "msvcrt.dll" && !strings.HasPrefix(name, "api-ms-win-") {
			t.Fatalf("jpegtran requires non-system runtime: %s", library)
		}
	}
}

func TestJPEGOptimizationLockedFileRetainsOriginal(t *testing.T) {
	directory := jpegTestToolDirectory(t)
	input := jpegOptimizationFixture(t, false)
	path := filepath.Join(t.TempDir(), "locked.jpg")
	writePNGTestFile(t, path, input)
	name, _ := syscall.UTF16PtrFromString(path)
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)
	result, err := OptimizeJPEG(context.Background(), JPEGOptimizationRequest{Path: path, EncoderDirectory: directory})
	if err == nil || result.Changed {
		t.Fatalf("expected replacement failure: %+v, %v", result, err)
	}
	output, _ := os.ReadFile(path)
	if !bytes.Equal(input, output) {
		t.Fatal("failed replacement lost original")
	}
}
