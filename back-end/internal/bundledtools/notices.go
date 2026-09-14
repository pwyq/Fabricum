package bundledtools

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

var noticeFiles = []string{
	"licenses/LICENSE",
	"licenses/THIRD-PARTY-NOTICES.md",
	"licenses/PATENTS.md",
	"licenses/NATIVE-TOOLS.md",
	"licenses/versions.json",
}

// Notices returns the legal and version inventory stored in a release binary.
func Notices() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate Fabricum executable: %w", err)
	}
	return noticesFrom(executable)
}

func noticesFrom(executable string) (string, error) {
	archive, err := zip.OpenReader(executable)
	if err != nil {
		if errors.Is(err, zip.ErrFormat) {
			return "", ErrNotBundled
		}
		return "", fmt.Errorf("open embedded notices: %w", err)
	}
	defer archive.Close()
	byName := make(map[string]*zip.File, len(archive.File))
	for _, file := range archive.File {
		byName[file.Name] = file
	}
	var output strings.Builder
	for _, name := range noticeFiles {
		file := byName[name]
		if file == nil {
			return "", fmt.Errorf("release binary is missing embedded notice %s", name)
		}
		reader, err := file.Open()
		if err != nil {
			return "", fmt.Errorf("open embedded notice %s: %w", name, err)
		}
		data, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return "", fmt.Errorf("read embedded notice %s: %w", name, err)
		}
		fmt.Fprintf(&output, "===== %s =====\n%s\n", name, strings.TrimSpace(string(data)))
	}
	return output.String(), nil
}
