package processing

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func replaceOptimizedPNG(ctx context.Context, path string, original, candidate []byte, info os.FileInfo) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".fabricum-png-*")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if _, err = temporary.Write(candidate); err == nil {
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
	current, err := os.Lstat(path)
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(info, current) {
		return fmt.Errorf("input file changed during optimization")
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, original) {
		return fmt.Errorf("input bytes changed during optimization")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return renameOptimizedPNG(temporary.Name(), path)
}
