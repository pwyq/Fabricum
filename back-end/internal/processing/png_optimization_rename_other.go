//go:build !windows

package processing

import "os"

func renameOptimizedPNG(source, destination string) error {
	return os.Rename(source, destination)
}
