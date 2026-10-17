package updater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type truncatedReader struct{}

func (truncatedReader) Read(p []byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestDownloadFailuresPreservePreviousInstallation(t *testing.T) {
	for _, scenario := range []string{"http-failure", "truncated", "timeout"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			previous := filepath.Join(root, "previous-version", "fabricum")
			if err := os.MkdirAll(filepath.Dir(previous), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(previous, []byte("working release"), 0755); err != nil {
				t.Fatal(err)
			}
			client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				switch scenario {
				case "http-failure":
					return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("failure")), Header: make(http.Header)}, nil
				case "truncated":
					return &http.Response{StatusCode: 200, Body: io.NopCloser(io.MultiReader(strings.NewReader("partial"), truncatedReader{})), Header: make(http.Header)}, nil
				default:
					<-r.Context().Done()
					return nil, r.Context().Err()
				}
			})}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			digest := fmt.Sprintf("%x", sha256.Sum256([]byte("new executable")))
			path, installed, err := install(ctx, newClient(client), root, "fabricum-new", digest, "https://github.com/binary")
			if err == nil || installed || path != "" {
				t.Fatalf("%s %v %v", path, installed, err)
			}
			data, _ := os.ReadFile(previous)
			if string(data) != "working release" {
				t.Fatal("lost previous release")
			}
			entries, _ := os.ReadDir(root)
			if len(entries) != 1 || entries[0].Name() != "previous-version" {
				t.Fatalf("partial download published: %v", entries)
			}
		})
	}
}

func TestChecksumSidecars(t *testing.T) {
	name := "fabricum-1.0.0-linux-x64"
	digest := strings.Repeat("ab", 32)
	for _, value := range []string{digest + "  " + name + "\n", strings.ToUpper(digest) + " *" + name + "\r\n"} {
		if got, err := parseChecksum([]byte(value), name); err != nil || got != digest {
			t.Fatalf("%s %v", got, err)
		}
	}
	for _, value := range []string{"", digest, digest + "  other", digest + "gg  " + name, "invalid  " + name, digest + "  " + name + "\n" + digest + "  other"} {
		if _, err := parseChecksum([]byte(value), name); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}

func TestIncompleteOrSymlinkInstallIsNotReplaced(t *testing.T) {
	root := t.TempDir()
	payload, name := []byte("new executable"), "fabricum-new"
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	destination := filepath.Join(root, name+"-"+digest)
	if err := os.MkdirAll(destination, 0700); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		t.Error("attempted download over an incomplete install")
		return nil, fmt.Errorf("unexpected request")
	})}
	if _, _, err := install(context.Background(), client, root, name, digest, "https://github.com/binary"); err == nil {
		t.Fatal("replaced incomplete directory")
	}
	if entries, _ := os.ReadDir(destination); len(entries) != 0 {
		t.Fatal("modified incomplete directory")
	}
	linkRoot := t.TempDir()
	link := filepath.Join(linkRoot, name+"-"+digest)
	if err := os.Symlink(destination, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, _, err := install(context.Background(), client, linkRoot, name, digest, "https://github.com/binary"); err == nil {
		t.Fatal("followed installation symlink")
	}
}

func TestMetadataSizeAndDiscoveryLimit(t *testing.T) {
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("x"), 4097))), Header: make(http.Header)}, nil
	})}
	if _, err := readResponse(context.Background(), client, "https://github.com/checksum", 4096); err == nil {
		t.Fatal("accepted oversized checksum")
	}
	requests := 0
	client = releaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprint(w, "["+strings.Repeat(`{"tag_name":"v1.0.0"},`, 99)+`{"tag_name":"v1.0.0"}]`)
	})
	if _, _, err := discover(context.Background(), newClient(client), bounds{}); err == nil || requests != 10 {
		t.Fatalf("discovery did not fail at its bound: requests=%d err=%v", requests, err)
	}
}
