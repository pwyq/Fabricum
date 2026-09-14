package processing

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func stageModelImages(document *modelDocument, directory string) error {
	images, err := modelArray(document.Object, "images")
	if err != nil {
		return err
	}
	for index, image := range images {
		uri, _ := modelString(image, "uri")
		if uri == "" || strings.HasPrefix(strings.ToLower(uri), "data:") {
			continue
		}
		decoded, err := url.PathUnescape(uri)
		if err != nil || strings.Contains(decoded, "://") {
			return fmt.Errorf("image %d has an unsupported URI", index)
		}
		source := filepath.Join(document.SourceDir, filepath.FromSlash(decoded))
		relative := filepath.FromSlash(decoded)
		if filepath.IsAbs(decoded) {
			source = decoded
			relative = filepath.Base(decoded)
		}
		destination := filepath.Join(directory, relative)
		if !modelPathInside(directory, destination) {
			return fmt.Errorf("image %d URI escapes the staging directory", index)
		}
		data, err := os.ReadFile(source)
		if err != nil {
			return fmt.Errorf("read image %d: %w", index, err)
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return fmt.Errorf("stage image %d: %w", index, err)
		}
		if err := os.WriteFile(destination, data, 0600); err != nil {
			return fmt.Errorf("stage image %d: %w", index, err)
		}
		if err := setModelValue(image, "uri", filepath.ToSlash(relative)); err != nil {
			return err
		}
	}
	return setModelArray(document.Object, "images", images)
}

func modelPathInside(directory, path string) bool {
	relative, err := filepath.Rel(directory, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func readModelOutputs(resultPath, outputPath string, request ModelOptimizationRequest, warnings []string) ([]ProcessedModelOutput, error) {
	mainData, err := os.ReadFile(resultPath)
	if err != nil {
		return nil, fmt.Errorf("read gltfpack output: %w", err)
	}
	format := modelOutputFormat(outputPath)
	facts, object, err := inspectModelOutput(resultPath, mainData, format)
	if err != nil {
		return nil, fmt.Errorf("validate gltfpack output: %w", err)
	}
	extensions := modelOutputExtensions(object)
	for _, extension := range extensions {
		if strings.Contains(strings.ToLower(extension), "draco") {
			return nil, errors.New("Draco output is not permitted")
		}
	}
	versions := modelNativeVersions(request)
	mainMeasurement := modelMeasurement(outputPath, format, mainData, facts, extensions, warnings, request.Role, versions)
	outputs := []ProcessedModelOutput{{Measurement: mainMeasurement, Path: outputPath, Data: mainData}}
	resources, err := modelOutputResources(resultPath, outputPath, object, request.Source)
	if err != nil {
		return nil, err
	}
	sort.Slice(resources, func(left, right int) bool { return resources[left].Path < resources[right].Path })
	for _, resource := range resources {
		measurement := modelMeasurement(resource.Path, resource.Format, resource.Data, AssetFacts{}, nil, nil, "", versions)
		outputs = append(outputs, ProcessedModelOutput{Measurement: measurement, Path: resource.Path, Data: resource.Data})
	}
	return outputs, nil
}

func inspectModelOutput(path string, data []byte, format string) (AssetFacts, map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	var facts AssetFacts
	var err error
	if format == "glb" {
		jsonData, _, parseErr := parseModelGLB(data)
		if parseErr != nil {
			return AssetFacts{}, nil, parseErr
		}
		object, err = decodeModelObject(jsonData)
		if err == nil {
			facts, err = inspectGLB(path, data)
		}
	} else {
		object, err = decodeModelObject(data)
		if err == nil {
			var binaryData []byte
			binaryData, err = modelGLTFBuffer(path, object)
			if err == nil {
				facts, err = inspectGLTF(path, data, binaryData)
			}
		}
	}
	if err != nil {
		return AssetFacts{}, nil, err
	}
	facts.Format = format
	facts.Bytes = int64(len(data))
	return facts, object, nil
}

func modelGLTFBuffer(path string, object map[string]json.RawMessage) ([]byte, error) {
	entries, err := modelArray(object, "buffers")
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}
	_, present := entries[0]["uri"]
	if !present {
		return nil, nil
	}
	value, err := modelString(entries[0], "uri")
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		return decodeDataURI(value)
	}
	decoded, err := url.PathUnescape(value)
	if err != nil || filepath.IsAbs(decoded) || strings.Contains(decoded, "://") {
		return nil, fmt.Errorf("gltf output has an unsupported buffer URI %q", value)
	}
	bufferPath := filepath.Join(filepath.Dir(path), filepath.FromSlash(decoded))
	if !modelPathInside(filepath.Dir(path), bufferPath) {
		return nil, fmt.Errorf("gltf output buffer URI %q escapes its workspace", value)
	}
	data, err := os.ReadFile(bufferPath)
	if err != nil {
		return nil, fmt.Errorf("read glTF output buffer %q: %w", value, err)
	}
	return data, nil
}
