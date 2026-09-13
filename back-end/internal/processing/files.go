package processing

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func WriteOutputs(outputs []ProcessedOutput) error {
	for _, output := range outputs {
		if err := WriteFileAtomically(output.Path, output.Data); err != nil {
			return fmt.Errorf("write %s output: %w", output.Measurement.Role, err)
		}
	}
	return nil
}

func WriteFileAtomically(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".unit-art-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.Write(data); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return os.Rename(temporaryPath, path)
	} else if err != nil {
		return err
	}
	backup, err := os.CreateTemp(directory, ".unit-art-backup-*")
	if err != nil {
		return err
	}
	backupPath := backup.Name()
	defer os.Remove(backupPath)
	if err := backup.Close(); err != nil {
		return err
	}
	if err := os.Remove(backupPath); err != nil {
		return err
	}
	if err := os.Rename(path, backupPath); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		_ = os.Rename(backupPath, path)
		return err
	}
	return os.Remove(backupPath)
}
