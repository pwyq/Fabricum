package releasebinary

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildCreatesOneExecutableWithOnlyRuntimePayload(t *testing.T) {
	for _, sample := range []struct {
		platform, suffix, library string
	}{
		{platform: "windows-x64", suffix: ".exe", library: "runtime.dll"},
		{platform: "linux-x64", library: "runtime.so.1"},
	} {
		t.Run(sample.platform, func(t *testing.T) {
			testBuild(t, sample.platform, sample.suffix, sample.library)
		})
	}
}

func testBuild(t *testing.T, platform, suffix, library string) {
	root := t.TempDir()
	native := filepath.Join(root, "native-output")
	if err := os.Mkdir(native, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range append(requiredTools, "decoder") {
		if err := os.WriteFile(filepath.Join(native, name+suffix), []byte(name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(native, library), []byte("library"), 0o600); err != nil {
		t.Fatal(err)
	}
	for source := range metadataFiles {
		path := filepath.Join(root, filepath.FromSlash(source))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	input := filepath.Join(root, "fabricum-input"+suffix)
	output := filepath.Join(root, "release", "fabricum"+suffix)
	if err := os.WriteFile(input, []byte("executable-prefix"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Build(Options{Input: input, NativeDirectory: native, Output: output, Root: root, Platform: platform}); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	names := make(map[string]bool)
	for _, file := range archive.File {
		names[file.Name] = true
	}
	for _, tool := range requiredTools {
		if !names["payload/"+tool+suffix] {
			t.Errorf("missing embedded %s", tool)
		}
	}
	if names["payload/decoder"+suffix] {
		t.Fatal("unneeded decoder was embedded")
	}
	if !names["payload/"+library] {
		t.Fatal("runtime library was not embedded")
	}
	if entries, err := os.ReadDir(filepath.Dir(output)); err != nil || len(entries) != 1 {
		t.Fatalf("release output is not one file: %v, %v", entries, err)
	}
}
