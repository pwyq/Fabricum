// Package updater discovers and installs verified standalone Fabricum releases.
package updater

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const DefaultTimeout = 2 * time.Minute

type Options struct {
	InstallDirectory string
	CurrentPath      string
	CurrentVersion   string
	MinVersion       string
	MaxVersion       string
	Timeout          time.Duration
}

type Result struct {
	SchemaVersion  int    `json:"schemaVersion"`
	Status         string `json:"status"`
	ExecutablePath string `json:"executablePath"`
	Version        string `json:"version"`
	Updated        bool   `json:"updated"`
	Error          string `json:"error,omitempty"`
}

// Update always checks GitHub before selecting an executable. Installs are
// immutable and versioned; the running binary and prior installs are preserved.
// A nil client uses the default HTTPS transport. A supplied client's transport
// may be used by embedded hosts; timeout and redirect restrictions still apply.
func Update(ctx context.Context, options Options, client *http.Client) (result Result, err error) {
	result.SchemaVersion, result.Status = 1, "failed"
	defer func() {
		if err != nil {
			result.Error = err.Error()
		}
	}()
	b, err := parseBounds(options.MinVersion, options.MaxVersion)
	if err != nil {
		return result, err
	}
	current, err := parseVersion(options.CurrentVersion, false)
	if err != nil {
		return result, err
	}
	platform, err := platformName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return result, err
	}
	if options.Timeout == 0 {
		options.Timeout = DefaultTimeout
	}
	if options.Timeout < 0 || options.Timeout > 10*time.Minute {
		return result, fmt.Errorf("timeout must be greater than zero and at most 10m")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	client = newClient(client)
	selected, latest, err := discover(ctx, client, b)
	if err != nil {
		return result, err
	}
	if b.contains(current) && current.compare(latest) >= 0 {
		path := options.CurrentPath
		if path == "" {
			path, err = os.Executable()
		}
		if err == nil {
			path, err = filepath.Abs(path)
		}
		if err != nil {
			return result, fmt.Errorf("locate current executable: %w", err)
		}
		result.Status, result.ExecutablePath, result.Version = "current", path, current.text
		return result, nil
	}
	name := "fabricum-" + latest.text + "-" + platform
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binaryURL, err := releaseAsset(selected, name)
	if err != nil {
		return result, err
	}
	checksumURL, err := releaseAsset(selected, name+".sha256")
	if err != nil {
		return result, err
	}
	checksum, err := readResponse(ctx, client, checksumURL, 4096)
	if err != nil {
		return result, fmt.Errorf("download checksum: %w", err)
	}
	digest, err := parseChecksum(checksum, name)
	if err != nil {
		return result, err
	}
	directory := options.InstallDirectory
	if directory == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return result, fmt.Errorf("locate update cache: %w", err)
		}
		directory = filepath.Join(cache, "fabricum", "releases")
	}
	directory, err = filepath.Abs(directory)
	if err != nil {
		return result, err
	}
	path, installed, err := install(ctx, client, directory, name, digest, binaryURL)
	if err != nil {
		return result, err
	}
	result.Status = "current"
	if installed {
		result.Status = "updated"
	}
	result.ExecutablePath, result.Version, result.Updated = path, latest.text, installed
	return result, nil
}
