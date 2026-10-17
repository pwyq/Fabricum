//go:build !windows

package processing

import "os"

func renameOptimizedFile(source, destination string) error {
	return os.Rename(source, destination)
}
