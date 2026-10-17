package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const releasesURL = "https://api.github.com/repos/pwyq/Fabricum/releases"

type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type release struct {
	Tag        string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []asset `json:"assets"`
}

func newClient(provided *http.Client) *http.Client {
	var client http.Client
	if provided != nil {
		client = *provided
	} else {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.ResponseHeaderTimeout = 15 * time.Second
		transport.TLSHandshakeTimeout = 10 * time.Second
		client.Transport = transport
	}
	if client.Timeout <= 0 || client.Timeout > 90*time.Second {
		client.Timeout = 90 * time.Second
	}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many release redirects")
		}
		u := request.URL
		if u.Scheme != "https" || u.User != nil || u.Port() != "" {
			return fmt.Errorf("unsafe release redirect")
		}
		switch u.Hostname() {
		case "api.github.com", "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
			return nil
		}
		return fmt.Errorf("unexpected release redirect host %q", u.Hostname())
	}
	return &client
}

func get(ctx context.Context, client *http.Client, address string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "Fabricum-Updater")
	if strings.HasPrefix(address, releasesURL) {
		request.Header.Set("Accept", "application/vnd.github+json")
		request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("release request failed: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("release request returned HTTP %d", response.StatusCode)
	}
	return response, nil
}

func readResponse(ctx context.Context, client *http.Client, address string, limit int64) ([]byte, error) {
	response, err := get(ctx, client, address)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read release response: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("release response exceeds %d bytes", limit)
	}
	return data, nil
}

func discover(ctx context.Context, client *http.Client, b bounds) (release, version, error) {
	var selected release
	var best version
	for page := 1; page <= 10; page++ {
		data, err := readResponse(ctx, client, fmt.Sprintf("%s?per_page=100&page=%d", releasesURL, page), 4<<20)
		if err != nil {
			return selected, best, fmt.Errorf("check releases: %w", err)
		}
		var releases []release
		if err := json.Unmarshal(data, &releases); err != nil {
			return selected, best, fmt.Errorf("decode releases: %w", err)
		}
		for _, item := range releases {
			if item.Draft || item.Prerelease || !strings.HasPrefix(item.Tag, "v") {
				continue
			}
			v, err := parseVersion(strings.TrimPrefix(item.Tag, "v"), true)
			if err == nil && b.contains(v) && (best.text == "" || v.compare(best) > 0) {
				selected, best = item, v
			}
		}
		if len(releases) < 100 {
			if best.text == "" {
				return selected, best, fmt.Errorf("no stable release satisfies the compatibility bounds")
			}
			return selected, best, nil
		}
	}
	return selected, best, fmt.Errorf("release discovery exceeded 1000 entries; check failed")
}

func platformName(operatingSystem, architecture string) (string, error) {
	if architecture != "amd64" || (operatingSystem != "windows" && operatingSystem != "linux") {
		return "", fmt.Errorf("release updates support only Windows x64 and Linux x64")
	}
	return operatingSystem + "-x64", nil
}

func releaseAsset(item release, name string) (string, error) {
	var address string
	for _, candidate := range item.Assets {
		if candidate.Name != name {
			continue
		}
		if address != "" {
			return "", fmt.Errorf("duplicate release asset %q", name)
		}
		address = candidate.URL
	}
	expected := "https://github.com/pwyq/Fabricum/releases/download/" + item.Tag + "/" + name
	u, err := url.Parse(address)
	if err != nil || address != expected || u == nil {
		return "", fmt.Errorf("release asset %q is missing or has an unexpected download URL", name)
	}
	return address, nil
}
