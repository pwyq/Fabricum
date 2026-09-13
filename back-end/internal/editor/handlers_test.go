package editor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fabricum/back-end/internal/processing"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportEndpointRequiresTokenAndWritesBothOutputs(t *testing.T) {
	temporary := t.TempDir()
	config := processorConfig{
		sourcePath: filepath.Join(temporary, "source.png"),
		sourceSize: 8,
		squareOutput: processing.OutputSpec{
			Role: "square", Path: filepath.Join(temporary, "square.png"), Width: 4, Height: 4,
		},
		wideOutput: processing.OutputSpec{
			Role: "wide", Path: filepath.Join(temporary, "wide.png"), Width: 4, Height: 3,
		},
	}
	writeFixtureImage(t, config.sourcePath, 8, 8)
	app, err := newApplication(config)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(app.handler())
	defer server.Close()
	for _, route := range []string{"/", "/app.js", "/app-utils.js", "/crop.js", "/source-selection.js", "/api/config", "/source"} {
		response, err := http.Get(server.URL + route)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("expected %s to return 200, got %d", route, response.StatusCode)
		}
	}
	payload, err := json.Marshal(processing.ExportRequest{
		Square: processing.CropRect{X: 1, Y: 1, Width: 6, Height: 6},
		Wide:   processing.CropRect{X: 0, Y: 1, Width: 8, Height: 6},
		Format: "png",
	})
	if err != nil {
		t.Fatal(err)
	}
	unauthorized, err := http.Post(server.URL+"/api/export", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	unauthorized.Body.Close()
	if unauthorized.StatusCode != http.StatusForbidden {
		t.Fatalf("expected forbidden without token, got %d", unauthorized.StatusCode)
	}
	previewRequest, err := http.NewRequest(http.MethodPost, server.URL+"/api/preview", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	previewRequest.Header.Set("Content-Type", "application/json")
	previewRequest.Header.Set("X-Unit-Art-Token", app.token)
	previewHTTPResponse, err := http.DefaultClient.Do(previewRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer previewHTTPResponse.Body.Close()
	if previewHTTPResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected successful preview, got %d", previewHTTPResponse.StatusCode)
	}
	var preview previewResponse
	if err := json.NewDecoder(previewHTTPResponse.Body).Decode(&preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Outputs) != 2 || !strings.HasPrefix(preview.DataURLs["square"], "data:image/png;base64,") {
		t.Fatalf("expected two PNG preview outputs, got %+v", preview)
	}
	if _, err := os.Stat(config.squareOutput.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preview must not write delivery files")
	}
	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/export", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Unit-Art-Token", app.token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected successful export, got %d", response.StatusCode)
	}
	assertImageDimensions(t, config.squareOutput.Path, 4, 4)
	assertImageDimensions(t, config.wideOutput.Path, 4, 3)
	info, err := os.Stat(config.squareOutput.Path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != int64(preview.Outputs[0].Bytes) {
		t.Fatalf("expected exported size %d to match preview size %d", info.Size(), preview.Outputs[0].Bytes)
	}
}

func TestSelectedRolePreviewsAndWritesOnlyThatOutput(t *testing.T) {
	temporary := t.TempDir()
	config := testProcessorConfig(temporary)
	writeFixtureImage(t, config.sourcePath, 8, 8)
	app, err := newApplication(config)
	if err != nil {
		t.Fatal(err)
	}
	input := processing.ExportRequest{
		Role:   "wide",
		Square: processing.CropRect{X: 1, Y: 1, Width: 6, Height: 6},
		Wide:   processing.CropRect{X: 0, Y: 1, Width: 8, Height: 6},
		Format: "png",
	}
	request := httptest.NewRequest(http.MethodPost, "http://localhost/api/preview", nil)
	outputs, err := app.previewOutputs(request, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(outputs) != 1 || outputs[0].Measurement.Role != "wide" {
		t.Fatalf("expected only the wide preview, got %+v", processing.OutputMeasurements(outputs))
	}
	if err := processing.WriteOutputs(outputs); err != nil {
		t.Fatal(err)
	}
	assertImageDimensions(t, config.wideOutput.Path, 4, 3)
	if _, err := os.Stat(config.squareOutput.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("selected wide export must not write the square output")
	}
}

func TestSelectedOutputSpecsRejectsUnknownRole(t *testing.T) {
	_, err := selectedOutputSpecs("portrait", processing.OutputSpec{Role: "square"}, processing.OutputSpec{Role: "wide"})
	if err == nil {
		t.Fatal("expected an unknown output role to fail")
	}
}

func TestConfigRejectsInvalidSourceReplacedAfterStartup(t *testing.T) {
	temporary := t.TempDir()
	config := testProcessorConfig(temporary)
	writeFixtureImage(t, config.sourcePath, 8, 8)
	app, err := newApplication(config)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureImage(t, config.sourcePath, 12, 12)
	request := httptest.NewRequest(http.MethodGet, "http://localhost/api/config", nil)
	response := httptest.NewRecorder()
	app.handler().ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected invalid replacement to fail config refresh, got %d", response.Code)
	}
}

func TestImportEndpointReplacesConfiguredSourcePNG(t *testing.T) {
	temporary := t.TempDir()
	config := testProcessorConfig(temporary)
	writeFixtureImage(t, config.sourcePath, 8, 8)
	app, err := newApplication(config)
	if err != nil {
		t.Fatal(err)
	}
	replacementPath := filepath.Join(temporary, "replacement.png")
	writeFixtureImageVariant(t, replacementPath, 8, 8, 7)
	replacement, err := os.ReadFile(replacementPath)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "http://localhost/api/import", bytes.NewReader(replacement))
	request.Header.Set("Content-Type", "image/png")
	request.Header.Set("X-Unit-Art-Token", app.token)
	response := httptest.NewRecorder()
	app.handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected successful import, got %d: %s", response.Code, response.Body.String())
	}
	var importedConfig clientConfig
	if err := json.NewDecoder(response.Body).Decode(&importedConfig); err != nil {
		t.Fatal(err)
	}
	if importedConfig.Source.Width != 8 || importedConfig.Source.Height != 8 {
		t.Fatalf("expected imported 8x8 source, got %dx%d", importedConfig.Source.Width, importedConfig.Source.Height)
	}
	info, err := os.Stat(config.sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if importedConfig.Source.Bytes != info.Size() {
		t.Fatalf("expected source size %d, got %d", info.Size(), importedConfig.Source.Bytes)
	}
	imported, err := os.ReadFile(config.sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(imported, replacement) {
		t.Fatal("expected import to preserve the selected PNG bytes")
	}
	assertImageDimensions(t, config.sourcePath, 8, 8)
}

func TestImportEndpointRejectsWrongSourceSize(t *testing.T) {
	temporary := t.TempDir()
	config := testProcessorConfig(temporary)
	writeFixtureImage(t, config.sourcePath, 8, 8)
	app, err := newApplication(config)
	if err != nil {
		t.Fatal(err)
	}
	invalidPath := filepath.Join(temporary, "invalid.png")
	writeFixtureImage(t, invalidPath, 12, 12)
	invalid, err := os.ReadFile(invalidPath)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "http://localhost/api/import", bytes.NewReader(invalid))
	request.Header.Set("Content-Type", "image/png")
	request.Header.Set("X-Unit-Art-Token", app.token)
	response := httptest.NewRecorder()
	app.handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected wrong source size to fail, got %d", response.Code)
	}
	assertImageDimensions(t, config.sourcePath, 8, 8)
}

func TestSourceUploadEndpointSelectsUploadedImage(t *testing.T) {
	temporary := t.TempDir()
	config, err := configure(Config{
		Mode:            ModeGUI,
		SourceSize:      8,
		SquareSize:      4,
		WideWidth:       4,
		OutputDirectory: temporary,
	})
	if err != nil {
		t.Fatal(err)
	}
	app, err := newApplication(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.cleanup)

	fixturePath := filepath.Join(temporary, "hero.png")
	writeFixtureImage(t, fixturePath, 8, 8)
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "http://localhost/api/source-upload", bytes.NewReader(fixture))
	request.Header.Set("Content-Type", "image/png")
	request.Header.Set("X-Unit-Art-Token", app.token)
	request.Header.Set("X-Fabricum-Source-Name", "hero.png")
	response := httptest.NewRecorder()
	app.handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected successful source upload, got %d: %s", response.Code, response.Body.String())
	}
	var uploaded clientConfig
	if err := json.NewDecoder(response.Body).Decode(&uploaded); err != nil {
		t.Fatal(err)
	}
	if uploaded.Source == nil || uploaded.Source.Width != 8 || uploaded.Source.Height != 8 {
		t.Fatalf("expected uploaded 8x8 source, got %+v", uploaded.Source)
	}
	if !strings.HasSuffix(uploaded.Source.Path, "hero.png") {
		t.Fatalf("expected uploaded filename to be retained, got %q", uploaded.Source.Path)
	}
	if _, err := os.Stat(app.config.sourcePath); err != nil {
		t.Fatal(err)
	}
}

func testProcessorConfig(directory string) processorConfig {
	return processorConfig{
		sourcePath: filepath.Join(directory, "source.png"),
		sourceSize: 8,
		squareOutput: processing.OutputSpec{
			Role: "square", Path: filepath.Join(directory, "square.png"), Width: 4, Height: 4,
		},
		wideOutput: processing.OutputSpec{
			Role: "wide", Path: filepath.Join(directory, "wide.png"), Width: 4, Height: 3,
		},
		encoderDirectory: filepath.Join(directory, "codecs"),
	}
}

func writeFixtureImage(t *testing.T, path string, width, height int) {
	writeFixtureImageVariant(t, path, width, height, 0)
}

func writeFixtureImageVariant(t *testing.T, path string, width, height, offset int) {
	t.Helper()
	fixture := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			fixture.SetNRGBA(x, y, color.NRGBA{R: uint8(x*20 + offset), G: uint8(y*30 + offset), B: uint8((x+y)*10 + offset), A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, fixture); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func assertImageDimensions(t *testing.T, path string, width, height int) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoded, _, err := image.DecodeConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Width != width || decoded.Height != height {
		t.Fatalf("expected %dx%d image, got %dx%d", width, height, decoded.Width, decoded.Height)
	}
}

func assertEncodedFormat(t *testing.T, path, format string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	isWebP := len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
	isAVIF := len(data) >= 12 && string(data[4:12]) == "ftypavif"
	if (format == "webp" && !isWebP) || (format == "avif" && !isAVIF) {
		t.Fatalf("expected valid %s header", format)
	}
}
