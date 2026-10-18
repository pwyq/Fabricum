package processing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func svgNewOutputPath(source, destination string) error {
	if strings.EqualFold(filepath.Clean(source), filepath.Clean(destination)) {
		return fmt.Errorf("SVG output must differ from the authoring source")
	}
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("SVG output already exists; choose a new destination")
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

// A hard link publishes a complete, synchronized file without replacing an
// existing destination (including symlinks or files created during optimization).
func writeNewSVG(ctx context.Context, destination string, data []byte, info os.FileInfo) error {
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".fabricum-svg-*")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if _, err = temporary.Write(data); err == nil {
		err = temporary.Chmod(info.Mode().Perm())
	}
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.Link(temporary.Name(), destination)
}
