//go:build windows

package processing

import (
	"bytes"
	"context"
	"debug/pe"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestPNGOptimizerWindowsRuntimeDependencies(t *testing.T) {
	directory := pngTestToolDirectory(t)
	file, err := pe.Open(nativeToolPath(directory, "oxipng"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	symbols, err := file.ImportedSymbols()
	if err != nil {
		t.Fatal(err)
	}
	system := map[string]bool{"kernel32.dll": true, "ntdll.dll": true, "bcryptprimitives.dll": true, "bcrypt.dll": true, "msvcrt.dll": true, "advapi32.dll": true, "ws2_32.dll": true, "userenv.dll": true}
	for _, symbol := range symbols {
		parts := strings.Split(symbol, ":")
		library := strings.ToLower(parts[len(parts)-1])
		if !system[library] && !strings.HasPrefix(library, "api-ms-win-") {
			t.Fatalf("oxipng requires an unbundled non-system runtime library: %s", library)
		}
	}
}

func TestPNGOptimizationLockedFileRetainsOriginal(t *testing.T) {
	input := pngOptimizationFixtures(t)["rgb"]
	path := filepath.Join(t.TempDir(), "locked.png")
	writePNGTestFile(t, path, input)
	name, _ := syscall.UTF16PtrFromString(path)
	// Simulate an application holding the file without permitting replacement.
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)
	pixels, _ := png.Decode(bytes.NewReader(input))
	var candidate bytes.Buffer
	_ = (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&candidate, pixels)
	result, err := optimizePNG(context.Background(), PNGOptimizationRequest{Path: path}, pngTestOptimizer(candidate.Bytes()))
	if err == nil || result.Changed {
		t.Fatalf("expected replacement failure: %+v, %v", result, err)
	}
	output, _ := os.ReadFile(path)
	if !bytes.Equal(input, output) {
		t.Fatal("failed replacement lost original PNG")
	}
}
