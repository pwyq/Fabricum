package bundledtools

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractsAppendedPayloadAndRepairsCache(t *testing.T) {
	executable := appendedArchive(t, map[string]string{
		"payload/cwebp.exe":   "encoder",
		"payload/runtime.dll": "library",
		"licenses/NOTICE.md":  "notice",
	})
	cache := t.TempDir()
	directory, err := extract(executable, cache)
	if err != nil {
		t.Fatal(err)
	}
	tool := filepath.Join(directory, "cwebp.exe")
	if data, err := os.ReadFile(tool); err != nil || string(data) != "encoder" {
		t.Fatalf("unexpected extracted tool %q: %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(directory, "NOTICE.md")); !os.IsNotExist(err) {
		t.Fatalf("notice should remain inside the executable: %v", err)
	}
	if err := os.WriteFile(tool, []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := extract(executable, cache); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(tool); string(data) != "encoder" {
		t.Fatalf("cache was not repaired: %q", data)
	}
}

func TestExecutableWithoutPayloadIsNotBundled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fabricum")
	if err := os.WriteFile(path, []byte("not a zip"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := extract(path, t.TempDir()); !errors.Is(err, ErrNotBundled) {
		t.Fatalf("expected ErrNotBundled, got %v", err)
	}
}

func TestReadsNoticesWithoutExtractingThem(t *testing.T) {
	files := map[string]string{"payload/cwebp": "encoder"}
	for _, name := range noticeFiles {
		files[name] = "contents of " + name
	}
	executable := appendedArchive(t, files)
	notices, err := noticesFrom(executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range noticeFiles {
		if !strings.Contains(notices, name) {
			t.Errorf("notice output omitted %s", name)
		}
	}
}

func appendedArchive(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fabricum.exe")
	output, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := output.WriteString("executable-prefix"); err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(output)
	archive.SetOffset(int64(len("executable-prefix")))
	for name, data := range files {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
