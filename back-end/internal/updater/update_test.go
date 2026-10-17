package updater

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func updateFixture(t *testing.T, payload []byte, checksum string) (*http.Client, Options, *atomic.Int32) {
	t.Helper()
	root := t.TempDir()
	currentPath := filepath.Join(root, "working-fabricum")
	if err := os.WriteFile(currentPath, []byte("working executable"), 0755); err != nil {
		t.Fatal(err)
	}
	options := Options{InstallDirectory: filepath.Join(root, "releases"), CurrentPath: currentPath, CurrentVersion: "1.0.0"}
	platform, _ := platformName(runtime.GOOS, runtime.GOARCH)
	name := "fabricum-2.0.0-" + platform
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	base := "https://github.com/pwyq/Fabricum/releases/download/v2.0.0/"
	if checksum == "" {
		checksum = fmt.Sprintf("%x  %s\n", sha256.Sum256(payload), name)
	}
	requests := new(atomic.Int32)
	client := releaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/pwyq/Fabricum/releases" {
			json.NewEncoder(w).Encode([]release{{Tag: "v2.0.0", Assets: []asset{{name, base + name}, {name + ".sha256", base + name + ".sha256"}}}})
		} else if strings.HasSuffix(r.URL.Path, ".sha256") {
			fmt.Fprint(w, checksum)
		} else {
			requests.Add(1)
			w.Write(payload)
		}
	})
	return client, options, requests
}

func TestUpdateInstallReuseAndCurrent(t *testing.T) {
	payload := []byte("verified release executable")
	client, options, requests := updateFixture(t, payload, "")
	first, err := Update(context.Background(), options, client)
	if err != nil || !first.Updated || first.Status != "updated" || first.Version != "2.0.0" || !filepath.IsAbs(first.ExecutablePath) {
		t.Fatalf("%+v %v", first, err)
	}
	data, _ := os.ReadFile(first.ExecutablePath)
	if string(data) != string(payload) {
		t.Fatal("installed incorrect bytes")
	}
	second, err := Update(nil, options, client)
	if err != nil || second.Updated || second.Status != "current" || second.ExecutablePath != first.ExecutablePath || requests.Load() != 1 {
		t.Fatalf("%+v %v requests=%d", second, err, requests.Load())
	}
	options.CurrentVersion, options.CurrentPath = "2.0.0", first.ExecutablePath
	third, err := Update(nil, options, client)
	if err != nil || third.Updated || third.ExecutablePath != first.ExecutablePath {
		t.Fatalf("%+v %v", third, err)
	}
	if err := os.WriteFile(first.ExecutablePath, []byte("tampered"), 0755); err != nil {
		t.Fatal(err)
	}
	options.CurrentVersion = "1.0.0"
	failure, err := Update(nil, options, client)
	if err == nil || failure.Status != "failed" || failure.Error == "" {
		t.Fatalf("reused corrupt cache: %+v %v", failure, err)
	}
}

func TestUpdateFailuresPreserveWorkingExecutable(t *testing.T) {
	for _, scenario := range []string{"checksum", "checksum-name", "empty-download", "no-compatible-release", "install-failure", "invalid-bounds", "timeout"} {
		t.Run(scenario, func(t *testing.T) {
			checksum := ""
			if scenario == "checksum" {
				checksum = strings.Repeat("0", 64) + "  fabricum-2.0.0-" + runtime.GOOS + "-x64"
				if runtime.GOOS == "windows" {
					checksum += ".exe"
				}
			}
			if scenario == "checksum-name" {
				checksum = strings.Repeat("0", 64) + "  wrong-binary"
			}
			payload := []byte("new executable")
			if scenario == "empty-download" {
				payload = nil
			}
			client, options, _ := updateFixture(t, payload, checksum)
			switch scenario {
			case "no-compatible-release":
				options.MaxVersion = "2.0.0"
			case "install-failure":
				options.InstallDirectory = options.CurrentPath
			case "invalid-bounds":
				options.MinVersion = "invalid"
			case "timeout":
				options.Timeout = time.Nanosecond
			}
			result, err := Update(nil, options, client)
			if err == nil || result.Status != "failed" || result.Updated || result.ExecutablePath != "" || result.Error == "" {
				t.Fatalf("%+v %v", result, err)
			}
			original, _ := os.ReadFile(options.CurrentPath)
			if string(original) != "working executable" {
				t.Fatal("lost working executable")
			}
			if scenario != "install-failure" {
				entries, _ := os.ReadDir(options.InstallDirectory)
				if len(entries) != 0 {
					t.Fatalf("left partial installation: %v", entries)
				}
			}
		})
	}
}

func TestConcurrentUpdates(t *testing.T) {
	client, options, _ := updateFixture(t, []byte("release"), "")
	if err := os.MkdirAll(filepath.Join(options.InstallDirectory, ".update-interrupted"), 0700); err != nil {
		t.Fatal(err)
	}
	const count = 16
	results := make(chan Result, count)
	failures := make(chan error, count)
	var workers sync.WaitGroup
	for range count {
		workers.Go(func() { result, err := Update(nil, options, client); results <- result; failures <- err })
	}
	workers.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	var selected string
	updated := 0
	for result := range results {
		if result.Updated {
			updated++
		}
		if selected == "" {
			selected = result.ExecutablePath
		}
		if result.ExecutablePath != selected {
			t.Fatalf("concurrent installs disagree: %+v", result)
		}
	}
	if updated != 1 {
		t.Fatalf("expected one publisher, got %d", updated)
	}
	data, _ := os.ReadFile(selected)
	if string(data) != "release" {
		t.Fatal("corrupt concurrent installation")
	}
	entries, _ := os.ReadDir(options.InstallDirectory)
	if len(entries) != 2 {
		t.Fatalf("staging directories leaked: %v", entries)
	}
}

func TestCompatibilityBoundsCanSelectOlderThanRunningVersion(t *testing.T) {
	client, options, _ := updateFixture(t, []byte("release"), "")
	options.CurrentVersion, options.MinVersion, options.MaxVersion = "3.0.0", "2.0.0", "3.0.0"
	result, err := Update(nil, options, client)
	if err != nil || result.Version != "2.0.0" {
		t.Fatalf("%+v %v", result, err)
	}
	options.MaxVersion = ""
	result, err = Update(nil, options, client)
	if err != nil || result.Version != "3.0.0" || result.Updated {
		t.Fatalf("downgraded compatible current executable: %+v %v", result, err)
	}
}

func TestUpdateNetworkTimeoutAndCancellation(t *testing.T) {
	client := releaseClient(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	start := time.Now()
	result, err := Update(nil, Options{CurrentVersion: "1.0.0", Timeout: 30 * time.Millisecond}, client)
	if err == nil || result.Status != "failed" || time.Since(start) > 2*time.Second {
		t.Fatalf("unbounded timeout: %+v %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Update(ctx, Options{CurrentVersion: "1.0.0"}, client); err == nil {
		t.Fatal("ignored cancellation")
	}
	for _, timeout := range []time.Duration{-time.Second, 11 * time.Minute} {
		if _, err := Update(nil, Options{CurrentVersion: "1.0.0", Timeout: timeout}, client); err == nil {
			t.Fatal("accepted invalid timeout")
		}
	}
}
