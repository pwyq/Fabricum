package processing

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type modelResource struct {
	Path   string
	Format string
	Data   []byte
}

func modelOutputResources(resultPath, outputPath string, object map[string]json.RawMessage, sourcePath string) ([]modelResource, error) {
	uris := make(map[string]struct{})
	for _, key := range []string{"buffers", "images"} {
		entries, err := modelArray(object, key)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			uri, _ := modelString(entry, "uri")
			if uri != "" && !strings.HasPrefix(strings.ToLower(uri), "data:") {
				uris[uri] = struct{}{}
			}
		}
	}
	values := make([]string, 0, len(uris))
	for uri := range uris {
		values = append(values, uri)
	}
	resultDirectory := filepath.Dir(resultPath)
	outputDirectory := filepath.Dir(outputPath)
	resources := make([]modelResource, 0, len(values))
	for _, uri := range values {
		decoded, err := url.PathUnescape(uri)
		if err != nil || filepath.IsAbs(decoded) || strings.Contains(decoded, "://") {
			return nil, fmt.Errorf("gltfpack output has an unsupported external URI %q", uri)
		}
		input := filepath.Join(resultDirectory, filepath.FromSlash(decoded))
		if !modelPathInside(resultDirectory, input) {
			return nil, fmt.Errorf("gltfpack output resource %q escapes its workspace", uri)
		}
		data, err := os.ReadFile(input)
		if err != nil {
			return nil, fmt.Errorf("read glTF output resource %q: %w", uri, err)
		}
		path := filepath.Join(outputDirectory, filepath.FromSlash(decoded))
		if sameTransformPath(path, sourcePath) || !modelPathInside(outputDirectory, path) {
			return nil, fmt.Errorf("gltfpack output resource %q conflicts with an input or output boundary", uri)
		}
		resources = append(resources, modelResource{Path: path, Format: modelSidecarFormat(path), Data: data})
	}
	return resources, nil
}

func modelSidecarFormat(path string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if ext == "" {
		return "bin"
	}
	return ext
}
