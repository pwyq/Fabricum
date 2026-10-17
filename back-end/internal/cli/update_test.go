package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fabricum/back-end/internal/editor"
	"fabricum/back-end/internal/updater"
)

type updateTransport func(*http.Request) (*http.Response, error)

func (f updateTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func updateTestClient(address string) *http.Client {
	server, _ := url.Parse(address)
	return &http.Client{Transport: updateTransport(func(request *http.Request) (*http.Response, error) {
		copy := request.Clone(request.Context())
		copy.URL.Scheme, copy.URL.Host = server.Scheme, server.Host
		return http.DefaultTransport.RoundTrip(copy)
	})}
}

func updateTestServer(t *testing.T, scenario string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	requests := new(atomic.Int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if scenario == "network-failure" {
			w.WriteHeader(503)
			return
		}
		if scenario == "timeout" {
			<-r.Context().Done()
			return
		}
		if strings.HasPrefix(r.URL.Path, "/repos/") {
			tag := "v99.0.0"
			if scenario == "current" {
				tag = "v0.0.1"
			}
			var releases []map[string]any
			for _, item := range []struct {
				version           string
				draft, prerelease bool
			}{{tag, false, false}, {"v100.0.0", true, false}, {"v101.0.0", false, true}, {"v102.0.0-rc.1", false, false}, {"v98.9.9", false, false}} {
				if scenario == "current" && item.version == "v98.9.9" {
					continue
				}
				name := "fabricum-" + strings.TrimPrefix(item.version, "v") + "-" + runtime.GOOS + "-x64"
				if runtime.GOOS == "windows" {
					name += ".exe"
				}
				base := "https://github.com/pwyq/Fabricum/releases/download/" + item.version + "/"
				assets := []map[string]string{{"name": name, "browser_download_url": base + name}, {"name": name + ".sha256", "browser_download_url": base + name + ".sha256"}}
				if scenario == "missing-platform" {
					assets = nil
				}
				releases = append(releases, map[string]any{"tag_name": item.version, "draft": item.draft, "prerelease": item.prerelease, "assets": assets})
			}
			json.NewEncoder(w).Encode(releases)
			return
		}
		if strings.HasSuffix(r.URL.Path, ".sha256") {
			name := strings.TrimSuffix(filepath.Base(r.URL.Path), ".sha256")
			digest := fmt.Sprintf("%x", sha256.Sum256([]byte("verified release")))
			if scenario == "checksum-failure" {
				digest = strings.Repeat("0", 64)
			}
			fmt.Fprintf(w, "%s  %s\n", digest, name)
			return
		}
		requests.Add(1)
		fmt.Fprint(w, "verified release")
	}))
	t.Cleanup(server.Close)
	return server, requests
}

func decodeUpdateResult(t *testing.T, data []byte) updater.Result {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	var result updater.Result
	if err := decoder.Decode(&result); err != nil {
		t.Fatalf("invalid JSON %q: %v", data, err)
	}
	if result.SchemaVersion != 1 {
		t.Fatalf("invalid schema: %+v", result)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		t.Fatalf("unexpected trailing output: %q", data)
	}
	return result
}

func TestUpdateCLIResults(t *testing.T) {
	for _, scenario := range []string{"update", "current", "network-failure", "checksum-failure", "missing-platform", "timeout", "bounds"} {
		t.Run(scenario, func(t *testing.T) {
			server, requests := updateTestServer(t, scenario)
			directory := t.TempDir()
			working := filepath.Join(directory, "working.exe")
			if err := os.WriteFile(working, []byte("working executable"), 0755); err != nil {
				t.Fatal(err)
			}
			args := []string{"--install-directory", filepath.Join(directory, "releases")}
			if scenario == "timeout" {
				args = append(args, "--timeout", "30ms")
			}
			if scenario == "bounds" {
				args = append(args, "--min-version", "98.0.0", "--max-version", "99.0.0")
			}
			var output bytes.Buffer
			err := runUpdateArgs(context.Background(), args, &output, updateTestClient(server.URL))
			result := decodeUpdateResult(t, output.Bytes())
			switch scenario {
			case "update", "bounds":
				wantVersion := "99.0.0"
				if scenario == "bounds" {
					wantVersion = "98.9.9"
				}
				if err != nil || result.Status != "updated" || !result.Updated || result.Version != wantVersion {
					t.Fatalf("%+v %v", result, err)
				}
				data, _ := os.ReadFile(result.ExecutablePath)
				if string(data) != "verified release" || !filepath.IsAbs(result.ExecutablePath) {
					t.Fatalf("bad executable: %+v", result)
				}
				output.Reset()
				if err := runUpdateArgs(nil, args, &output, updateTestClient(server.URL)); err != nil {
					t.Fatal(err)
				}
				reuse := decodeUpdateResult(t, output.Bytes())
				if reuse.Updated || reuse.Status != "current" || reuse.ExecutablePath != result.ExecutablePath || requests.Load() != 1 {
					t.Fatalf("bad reuse: %+v", reuse)
				}
			case "current":
				if err != nil || result.Status != "current" || result.Updated || result.Version != editor.Version || result.ExecutablePath == "" || requests.Load() != 0 {
					t.Fatalf("%+v %v", result, err)
				}
				if _, err := os.Stat(filepath.Join(directory, "releases")); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("no-update created install directory")
				}
			default:
				if err == nil || result.Status != "failed" || result.Updated || result.Error == "" || result.ExecutablePath != "" {
					t.Fatalf("%+v %v", result, err)
				}
				entries, _ := os.ReadDir(filepath.Join(directory, "releases"))
				if len(entries) != 0 {
					t.Fatalf("partial installation: %v", entries)
				}
			}
			original, _ := os.ReadFile(working)
			if string(original) != "working executable" {
				t.Fatal("replaced previous executable")
			}
		})
	}
}

func TestUpdateCLIArgumentErrorsAndHelp(t *testing.T) {
	for _, args := range [][]string{{"--unknown"}, {"extra"}, {"--min-version", "bad"}, {"--min-version", "2.0.0", "--max-version", "1.0.0"}, {"--timeout", "0"}, {"--timeout", "-1s"}, {"--timeout", "11m"}, {"--timeout", "invalid"}, {"--install-directory"}} {
		var output bytes.Buffer
		if err := RunUpdateArgs(nil, args, &output); err == nil {
			t.Fatalf("accepted %v", args)
		}
		result := decodeUpdateResult(t, output.Bytes())
		if result.Status != "failed" || result.Error == "" {
			t.Fatalf("%+v", result)
		}
	}
	var help bytes.Buffer
	if err := RunUpdateArgs(nil, []string{"--help"}, &help); !errors.Is(err, flag.ErrHelp) || !strings.Contains(help.String(), "fabricum update") {
		t.Fatalf("%s %v", help.String(), err)
	}
}

// Each helper is an independent process invoking the same CLI entry used by
// main. Only its HTTP transport is redirected to the local fixture server.
func TestUpdateCLIProcessHelper(t *testing.T) {
	address := os.Getenv("FABRICUM_TEST_UPDATE_SERVER")
	if address == "" {
		return
	}
	err := runUpdateArgs(nil, []string{"--install-directory", os.Getenv("FABRICUM_TEST_UPDATE_DIRECTORY")}, os.Stdout, updateTestClient(address))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func TestUpdateCLIConcurrentProcesses(t *testing.T) {
	server, _ := updateTestServer(t, "update")
	directory := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	const count = 4
	var workers sync.WaitGroup
	outputs := make([][]byte, count)
	errors := make([]error, count)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for i := range count {
		workers.Go(func() {
			command := exec.CommandContext(ctx, executable, "-test.run=^TestUpdateCLIProcessHelper$")
			command.Env = append(os.Environ(), "FABRICUM_TEST_UPDATE_SERVER="+server.URL, "FABRICUM_TEST_UPDATE_DIRECTORY="+directory)
			outputs[i], errors[i] = command.CombinedOutput()
		})
	}
	workers.Wait()
	updated, selected := 0, ""
	for i, data := range outputs {
		if errors[i] != nil {
			t.Fatalf("child failed: %s %v", data, errors[i])
		}
		result := decodeUpdateResult(t, data)
		if result.Updated {
			updated++
		}
		if selected == "" {
			selected = result.ExecutablePath
		}
		if result.ExecutablePath != selected {
			t.Fatalf("processes selected different paths: %+v", result)
		}
	}
	if updated != 1 {
		t.Fatalf("expected one publisher, got %d", updated)
	}
	data, _ := os.ReadFile(selected)
	if string(data) != "verified release" {
		t.Fatal("corrupted install")
	}
	entries, _ := os.ReadDir(directory)
	if len(entries) != 1 {
		t.Fatalf("incomplete staging directories: %v", entries)
	}
}
