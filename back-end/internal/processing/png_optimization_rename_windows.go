//go:build windows

package processing

import (
	"syscall"
	"unsafe"
)

var pngMoveFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

func renameOptimizedPNG(source, destination string) error {
	from, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := syscall.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	// Same-directory replacement; no delete/backup gap. WRITE_THROUGH completes
	// the move before returning. A failed move keeps the existing destination.
	result, _, callErr := pngMoveFileEx.Call(uintptr(unsafe.Pointer(from)), uintptr(unsafe.Pointer(to)), 0x1|0x8)
	if result == 0 {
		return callErr
	}
	return nil
}
