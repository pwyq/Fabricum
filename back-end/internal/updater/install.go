package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const maxBinaryBytes int64 = 128 << 20

func parseChecksum(data []byte, name string) (string, error) {
	fields := strings.Fields(string(data))
	if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
		return "", fmt.Errorf("checksum must contain exactly one SHA-256 entry for %s", name)
	}
	digest, err := hex.DecodeString(fields[0])
	if err != nil || len(digest) != sha256.Size {
		return "", fmt.Errorf("invalid published SHA-256")
	}
	return hex.EncodeToString(digest), nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func verifyInstalled(ctx context.Context, directory, name, digest string) error {
	info, err := os.Lstat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("installation directory is not a regular directory")
	}
	path := filepath.Join(directory, name)
	info, err = os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0) {
		return fmt.Errorf("installed executable is not a regular executable file")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(contextReader{ctx, file}, maxBinaryBytes+1))
	if err != nil {
		return err
	}
	if n == 0 || n > maxBinaryBytes || hex.EncodeToString(hash.Sum(nil)) != digest {
		return fmt.Errorf("installed executable failed SHA-256 verification")
	}
	return nil
}

func install(ctx context.Context, client *http.Client, root, name, digest, address string) (string, bool, error) {
	// The content hash also isolates a republished asset from an earlier install.
	destination := filepath.Join(root, name+"-"+digest)
	path := filepath.Join(destination, name)
	if _, err := os.Lstat(destination); err == nil {
		if err := verifyInstalled(ctx, destination, name, digest); err != nil {
			return "", false, fmt.Errorf("verify existing installation: %w", err)
		}
		return path, false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", false, fmt.Errorf("inspect existing installation: %w", err)
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", false, fmt.Errorf("create update directory: %w", err)
	}
	stage, err := os.MkdirTemp(root, ".update-")
	if err != nil {
		return "", false, err
	}
	defer os.RemoveAll(stage)
	if err := download(ctx, client, address, filepath.Join(stage, name), digest); err != nil {
		return "", false, err
	}
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	// One same-filesystem directory rename publishes only a complete, verified
	// executable. Concurrent winners are verified and reused; no lock can go stale.
	if err := os.Rename(stage, destination); err != nil {
		if checkErr := verifyInstalled(ctx, destination, name, digest); checkErr == nil {
			return path, false, nil
		}
		return "", false, fmt.Errorf("publish verified installation: %w", err)
	}
	return path, true, nil
}

func download(ctx context.Context, client *http.Client, address, path, digest string) (err error) {
	response, err := get(ctx, client, address)
	if err != nil {
		return fmt.Errorf("download executable: %w", err)
	}
	defer response.Body.Close()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(contextReader{ctx, response.Body}, maxBinaryBytes+1))
	if err != nil {
		return fmt.Errorf("download executable: %w", err)
	}
	if n == 0 || n > maxBinaryBytes {
		return fmt.Errorf("release executable size is outside 1..%d bytes", maxBinaryBytes)
	}
	if hex.EncodeToString(hash.Sum(nil)) != digest {
		return fmt.Errorf("downloaded executable failed SHA-256 verification")
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync verified executable: %w", err)
	}
	return file.Chmod(0755)
}
