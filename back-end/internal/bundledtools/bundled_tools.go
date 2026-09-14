package bundledtools

import (
	"archive/zip"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

var ErrNotBundled = errors.New("native tools are not embedded")

var requiredTools = map[string]struct{}{
	"avifenc": {}, "basisu": {}, "cwebp": {}, "gltfpack": {},
}

var resolved struct {
	sync.Once
	directory string
	err       error
}

// Tool returns an extracted native tool from the payload appended to Fabricum.
func Tool(name string) (string, error) {
	if _, ok := requiredTools[name]; !ok {
		return "", fmt.Errorf("unsupported embedded native tool %q", name)
	}
	resolved.Do(func() {
		executable, err := os.Executable()
		if err != nil {
			resolved.err = fmt.Errorf("locate Fabricum executable: %w", err)
			return
		}
		if err := payloadAvailable(executable); err != nil {
			resolved.err = err
			return
		}
		cache, err := os.UserCacheDir()
		if err != nil {
			resolved.err = fmt.Errorf("locate native tool cache: %w", err)
			return
		}
		resolved.directory, resolved.err = extract(executable, filepath.Join(cache, "fabricum", "native"))
	})
	if resolved.err != nil {
		return "", resolved.err
	}
	path := filepath.Join(resolved.directory, toolFilename(name, runtime.GOOS))
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("embedded native tool %s is unavailable: %w", name, err)
	}
	return path, nil
}

func payloadAvailable(executable string) error {
	archive, err := zip.OpenReader(executable)
	if err != nil {
		if errors.Is(err, zip.ErrFormat) {
			return ErrNotBundled
		}
		return fmt.Errorf("open embedded native payload: %w", err)
	}
	defer archive.Close()
	for _, file := range archive.File {
		if strings.HasPrefix(file.Name, "payload/") && !file.FileInfo().IsDir() {
			return nil
		}
	}
	return ErrNotBundled
}

func extract(executable, cacheRoot string) (string, error) {
	archive, err := zip.OpenReader(executable)
	if err != nil {
		if errors.Is(err, zip.ErrFormat) {
			return "", ErrNotBundled
		}
		return "", fmt.Errorf("open embedded native payload: %w", err)
	}
	defer archive.Close()
	files, digest, err := payloadFiles(archive.File)
	if err != nil {
		return "", err
	}
	directory := filepath.Join(cacheRoot, digest)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create native tool cache: %w", err)
	}
	for _, file := range files {
		if err := extractFile(file, filepath.Join(directory, filepath.Base(file.Name))); err != nil {
			return "", err
		}
	}
	return directory, nil
}

func payloadFiles(files []*zip.File) ([]*zip.File, string, error) {
	selected := make([]*zip.File, 0)
	seen := make(map[string]struct{})
	for _, file := range files {
		if !strings.HasPrefix(file.Name, "payload/") || file.FileInfo().IsDir() {
			continue
		}
		name := strings.TrimPrefix(file.Name, "payload/")
		if name == "" || filepath.Base(name) != name {
			return nil, "", fmt.Errorf("invalid embedded native path %q", file.Name)
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, "", fmt.Errorf("duplicate embedded native file %q", name)
		}
		seen[name] = struct{}{}
		selected = append(selected, file)
	}
	if len(selected) == 0 {
		return nil, "", ErrNotBundled
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Name < selected[j].Name })
	hash := sha256.New()
	for _, file := range selected {
		io.WriteString(hash, file.Name)
		fmt.Fprintf(hash, ":%d:%08x\n", file.UncompressedSize64, file.CRC32)
	}
	return selected, fmt.Sprintf("%x", hash.Sum(nil))[:24], nil
}

func extractFile(file *zip.File, destination string) error {
	if matchesFile(destination, file) {
		return nil
	}
	source, err := file.Open()
	if err != nil {
		return fmt.Errorf("open embedded %s: %w", file.Name, err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".fabricum-native-*")
	if err != nil {
		source.Close()
		return fmt.Errorf("create cached %s: %w", file.Name, err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	_, copyErr := io.Copy(temporary, source)
	closeSourceErr := source.Close()
	chmodErr := temporary.Chmod(0o700)
	closeErr := temporary.Close()
	if err := errors.Join(copyErr, closeSourceErr, chmodErr, closeErr); err != nil {
		return fmt.Errorf("extract embedded %s: %w", file.Name, err)
	}
	if err := os.Rename(temporaryName, destination); err != nil {
		if matchesFile(destination, file) {
			return nil
		}
		_ = os.Remove(destination)
		if retryErr := os.Rename(temporaryName, destination); retryErr != nil {
			return fmt.Errorf("install cached %s: %w", file.Name, err)
		}
	}
	return nil
}

func matchesFile(path string, expected *zip.File) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || uint64(info.Size()) != expected.UncompressedSize64 {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	hash := crc32.NewIEEE()
	if _, err := io.Copy(hash, file); err != nil {
		return false
	}
	return hash.Sum32() == expected.CRC32
}

func toolFilename(name, operatingSystem string) string {
	if operatingSystem == "windows" {
		return name + ".exe"
	}
	return name
}
