package releasebinary

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var requiredTools = []string{"avifenc", "basisu", "cwebp", "gltfpack"}

var metadataFiles = map[string]string{
	"LICENSE":              "licenses/LICENSE",
	"native/NOTICE.md":     "licenses/THIRD-PARTY-NOTICES.md",
	"native/PATENTS.md":    "licenses/PATENTS.md",
	"native/README.md":     "licenses/NATIVE-TOOLS.md",
	"native/versions.json": "licenses/versions.json",
}

type Options struct {
	Input, NativeDirectory, Output, Root, Platform string
}

// Build creates one executable containing its native tools and legal metadata.
func Build(options Options) error {
	if options.Platform != "windows-x64" && options.Platform != "linux-x64" {
		return fmt.Errorf("unsupported release platform %q", options.Platform)
	}
	if samePath(options.Input, options.Output) {
		return fmt.Errorf("release input and output must differ")
	}
	payload, err := collectPayload(options.NativeDirectory, options.Platform)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(options.Output), 0o755); err != nil {
		return fmt.Errorf("create release directory: %w", err)
	}
	input, err := os.Open(options.Input)
	if err != nil {
		return fmt.Errorf("open Fabricum executable: %w", err)
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return fmt.Errorf("inspect Fabricum executable: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(options.Output), ".fabricum-release-*")
	if err != nil {
		return fmt.Errorf("create release executable: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	written, err := io.Copy(temporary, input)
	if err != nil {
		temporary.Close()
		return fmt.Errorf("copy Fabricum executable: %w", err)
	}
	archive := zip.NewWriter(temporary)
	archive.SetOffset(written)
	for _, source := range payload {
		if err := addFile(archive, source.path, "payload/"+source.name, 0o700); err != nil {
			temporary.Close()
			return err
		}
	}
	metadataNames := make([]string, 0, len(metadataFiles))
	for name := range metadataFiles {
		metadataNames = append(metadataNames, name)
	}
	sort.Strings(metadataNames)
	for _, name := range metadataNames {
		if err := addFile(archive, filepath.Join(options.Root, filepath.FromSlash(name)), metadataFiles[name], 0o600); err != nil {
			temporary.Close()
			return err
		}
	}
	if err := archive.Close(); err != nil {
		temporary.Close()
		return fmt.Errorf("finish embedded payload: %w", err)
	}
	if err := temporary.Chmod(info.Mode()); err != nil {
		temporary.Close()
		return fmt.Errorf("preserve executable mode: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close release executable: %w", err)
	}
	if err := os.Remove(options.Output); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("replace release executable: %w", err)
	}
	if err := os.Rename(temporaryName, options.Output); err != nil {
		return fmt.Errorf("install release executable: %w", err)
	}
	return nil
}

type payloadFile struct{ name, path string }

func collectPayload(directory, platform string) ([]payloadFile, error) {
	suffix := ""
	if platform == "windows-x64" {
		suffix = ".exe"
	}
	files := make([]payloadFile, 0, len(requiredTools))
	for _, tool := range requiredTools {
		name := tool + suffix
		path := filepath.Join(directory, name)
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("missing required native tool %s", path)
		}
		files = append(files, payloadFile{name: name, path: path})
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read native tool directory: %w", err)
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && runtimeLibrary(entry.Name(), platform) {
			files = append(files, payloadFile{name: entry.Name(), path: filepath.Join(directory, entry.Name())})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return files, nil
}

func runtimeLibrary(name, platform string) bool {
	lower := strings.ToLower(name)
	if platform == "windows-x64" {
		return strings.HasSuffix(lower, ".dll")
	}
	return strings.Contains(lower, ".so.") || strings.HasSuffix(lower, ".so")
}

func addFile(archive *zip.Writer, source, name string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open release payload %s: %w", source, err)
	}
	defer input.Close()
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(mode)
	header.Modified = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	output, err := archive.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create release payload %s: %w", name, err)
	}
	if _, err := io.Copy(output, input); err != nil {
		return fmt.Errorf("write release payload %s: %w", name, err)
	}
	return nil
}

func samePath(first, second string) bool {
	left, _ := filepath.Abs(first)
	right, _ := filepath.Abs(second)
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}
