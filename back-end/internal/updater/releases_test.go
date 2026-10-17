package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func releaseClient(t *testing.T, handler http.HandlerFunc) *http.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	address, _ := url.Parse(server.URL)
	return &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		copy := r.Clone(r.Context())
		copy.URL.Scheme, copy.URL.Host = address.Scheme, address.Host
		return server.Client().Transport.RoundTrip(copy)
	})}
}

func TestVersionsAndBounds(t *testing.T) {
	ordered := []string{"0.2.9", "0.2.10-alpha.1", "0.2.10-alpha.2", "0.2.10-beta.0", "0.2.10-rc.0", "0.2.10", "1.0.0"}
	for i, text := range ordered {
		v, err := parseVersion(text, false)
		if err != nil {
			t.Fatal(err)
		}
		if v.compare(v) != 0 {
			t.Fatal("version differs from itself")
		}
		if i > 0 {
			before, _ := parseVersion(ordered[i-1], false)
			if before.compare(v) >= 0 || v.compare(before) <= 0 {
				t.Fatalf("wrong ordering: %s, %s", before.text, text)
			}
		}
	}
	for _, text := range []string{"", "v1.0.0", "1.0", "01.0.0", "1.0.0/../../evil", "1.0.0+build", "18446744073709551616.0.0", "1.0.0-alpha.18446744073709551616"} {
		if _, err := parseVersion(text, false); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
	for _, pair := range [][2]string{{"1.0.0", "1.0.0"}, {"2.0.0", "1.0.0"}, {"1.0.0-rc.1", ""}, {"", "bad"}} {
		if _, err := parseBounds(pair[0], pair[1]); err == nil {
			t.Fatalf("accepted bounds %v", pair)
		}
	}
	b, err := parseBounds("1.0.0", "2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	for text, want := range map[string]bool{"0.9.9": false, "1.0.0": true, "1.9.9": true, "2.0.0": false} {
		v, _ := parseVersion(text, true)
		if b.contains(v) != want {
			t.Fatalf("bounds for %s", text)
		}
	}
}

func TestDiscoverStableCompatibleReleaseAcrossPages(t *testing.T) {
	pages := 0
	client := releaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		pages++
		if r.Header.Get("Accept") != "application/vnd.github+json" || r.Header.Get("User-Agent") == "" {
			t.Error("missing API headers")
		}
		if r.URL.Query().Get("page") == "1" {
			items := make([]release, 100)
			items[0] = release{Tag: "v9.0.0", Draft: true}
			items[1] = release{Tag: "v8.0.0", Prerelease: true}
			items[2] = release{Tag: "v7.0.0-rc.1"}
			items[3] = release{Tag: "v3.0.0"}
			items[4] = release{Tag: "v1.2.0"}
			json.NewEncoder(w).Encode(items)
			return
		}
		json.NewEncoder(w).Encode([]release{{Tag: "v1.10.0"}, {Tag: "v1.3.0"}, {Tag: "v1.01.0"}})
	})
	b, _ := parseBounds("1.0.0", "2.0.0")
	item, v, err := discover(context.Background(), newClient(client), b)
	if err != nil || v.text != "1.10.0" || item.Tag != "v1.10.0" || pages != 2 {
		t.Fatalf("%+v %+v %v pages=%d", item, v, err, pages)
	}
}

func TestDiscoveryFailures(t *testing.T) {
	for name, body := range map[string]string{"empty": "[]", "malformed": "{", "trailing": "[] true", "oversized": strings.Repeat(" ", (4<<20)+1)} {
		t.Run(name, func(t *testing.T) {
			client := releaseClient(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) })
			if _, _, err := discover(context.Background(), newClient(client), bounds{}); err == nil {
				t.Fatal("expected check failure")
			}
		})
	}
	for _, status := range []int{403, 404, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			client := releaseClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) })
			if _, _, err := discover(context.Background(), newClient(client), bounds{}); err == nil || !strings.Contains(err.Error(), fmt.Sprint(status)) {
				t.Fatal(err)
			}
		})
	}
}

func TestPlatformsAndAssetURLs(t *testing.T) {
	for _, operatingSystem := range []string{"linux", "windows"} {
		if got, err := platformName(operatingSystem, "amd64"); err != nil || got != operatingSystem+"-x64" {
			t.Fatalf("%s %v", got, err)
		}
	}
	for _, pair := range [][2]string{{"darwin", "amd64"}, {"linux", "arm64"}, {"windows", "386"}} {
		if _, err := platformName(pair[0], pair[1]); err == nil {
			t.Fatalf("accepted %v", pair)
		}
	}
	name := "fabricum-1.0.0-linux-x64"
	address := "https://github.com/pwyq/Fabricum/releases/download/v1.0.0/" + name
	for _, bad := range []string{"", "http" + strings.TrimPrefix(address, "https"), address + "?redirect=evil", strings.Replace(address, "pwyq", "evil", 1)} {
		if _, err := releaseAsset(release{Tag: "v1.0.0", Assets: []asset{{name, bad}}}, name); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	if got, err := releaseAsset(release{Tag: "v1.0.0", Assets: []asset{{name, address}}}, name); err != nil || got != address {
		t.Fatal(got, err)
	}
	if _, err := releaseAsset(release{Tag: "v1.0.0", Assets: []asset{{name, address}, {name, address}}}, name); err == nil {
		t.Fatal("accepted duplicate asset")
	}
	client := newClient(nil)
	for _, address := range []string{"http://github.com/file", "https://evil.example/file", "https://github.com:443/file", "https://user@github.com/file"} {
		u, _ := url.Parse(address)
		if err := client.CheckRedirect(&http.Request{URL: u}, nil); err == nil {
			t.Fatalf("accepted redirect %s", address)
		}
	}
}
