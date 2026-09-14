package compatibility

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	fabricum "fabricum/back-end"
)

type imageResponse struct {
	SchemaVersion int                          `json:"schemaVersion"`
	Processor     string                       `json:"processor"`
	Source        string                       `json:"source"`
	Request       fabricum.ExportRequest       `json:"request"`
	Outputs       []fabricum.OutputMeasurement `json:"outputs"`
	DataURLs      map[string]string            `json:"dataUrls"`
}

func runImageWorkflow(ctx context.Context, fixture fixtures) ([]fabricum.ExportReceipt, error) {
	base := filepath.Join(fixture.Root, "exports", "square.png")
	wide := filepath.Join(fixture.Root, "exports", "wide.png")
	handler, err := fabricum.NewHandler(fabricum.Config{
		Mode: fabricum.ModeCLI, Source: fixture.Source, SourceSize: 1024, SquareSize: 512, WideWidth: 768,
		SquareOutput: base, WideOutput: wide,
	})
	if err != nil {
		return nil, fmt.Errorf("configure image workflow: %w", err)
	}
	token, err := handlerToken(handler)
	if err != nil {
		return nil, err
	}
	receipts := make([]fabricum.ExportReceipt, 0, 3)
	for _, format := range []string{"png", "webp", "avif"} {
		quality := 0
		if format != "png" {
			quality = 90
		}
		input := fabricum.ExportRequest{
			Square: fabricum.CropRect{Width: 1024, Height: 1024},
			Wide:   fabricum.CropRect{X: 0, Y: 128, Width: 1024, Height: 768},
			Format: format, Quality: quality,
		}
		payload, err := json.Marshal(input)
		if err != nil {
			return nil, err
		}
		preview, err := callImageEndpoint(ctx, handler, token, "/api/preview", payload)
		if err != nil {
			return nil, fmt.Errorf("%s preview: %w", format, err)
		}
		exported, err := callImageEndpoint(ctx, handler, token, "/api/export", payload)
		if err != nil {
			return nil, fmt.Errorf("%s export: %w", format, err)
		}
		if preview.SchemaVersion != 1 || exported.SchemaVersion != 1 || preview.Processor != exported.Processor || !reflect.DeepEqual(preview.Outputs, exported.Outputs) {
			return nil, fmt.Errorf("%s preview and export receipts differ", format)
		}
		if err := validateImageWorkflow(preview, exported, fixture.Root, format); err != nil {
			return nil, err
		}
		receipts = append(receipts, fabricum.ExportReceipt{
			SchemaVersion: exported.SchemaVersion, Processor: exported.Processor, Source: exported.Source,
			Request: exported.Request, Outputs: exported.Outputs,
		})
	}
	return receipts, nil
}

func handlerToken(handler http.Handler) (string, error) {
	request := httptest.NewRequest(http.MethodGet, "http://localhost/api/config", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		return "", fmt.Errorf("read editor config: status %d", response.Code)
	}
	var config struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&config); err != nil {
		return "", fmt.Errorf("decode editor config: %w", err)
	}
	if config.Token == "" {
		return "", fmt.Errorf("editor config did not provide a session token")
	}
	return config.Token, nil
}

func callImageEndpoint(ctx context.Context, handler http.Handler, token, route string, payload []byte) (imageResponse, error) {
	request := httptest.NewRequest(http.MethodPost, "http://localhost"+route, bytes.NewReader(payload)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Unit-Art-Token", token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		return imageResponse{}, fmt.Errorf("status %d: %s", response.Code, strings.TrimSpace(response.Body.String()))
	}
	var result imageResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return imageResponse{}, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}

func validateImageWorkflow(preview, exported imageResponse, root, format string) error {
	if len(preview.Outputs) != 2 || len(preview.DataURLs) != 2 {
		return fmt.Errorf("%s workflow returned %d outputs", format, len(preview.Outputs))
	}
	wants := map[string][2]int{"square": {512, 512}, "wide": {768, 576}}
	for _, output := range exported.Outputs {
		want, ok := wants[output.Role]
		if !ok || output.Format != format || output.Width != want[0] || output.Height != want[1] {
			return fmt.Errorf("%s workflow has unexpected measurement: %+v", format, output)
		}
		if err := checkImageMeasurement(output); err != nil {
			return err
		}
		encoded, err := decodeDataURL(preview.DataURLs[output.Role])
		if err != nil {
			return fmt.Errorf("%s preview %s: %w", format, output.Role, err)
		}
		written, err := os.ReadFile(filepath.FromSlash(output.Path))
		if err != nil {
			return fmt.Errorf("read exported %s: %w", output.Role, err)
		}
		if !bytes.Equal(encoded, written) {
			return fmt.Errorf("%s preview and exported %s bytes differ", format, output.Role)
		}
		if filepath.Dir(filepath.FromSlash(output.Path)) != filepath.Join(root, "exports") {
			return fmt.Errorf("%s output escaped fixture directory", output.Role)
		}
	}
	return nil
}

func decodeDataURL(value string) ([]byte, error) {
	comma := strings.IndexByte(value, ',')
	if comma < 0 || !strings.HasSuffix(strings.ToLower(value[:comma]), ";base64") {
		return nil, fmt.Errorf("invalid data URL")
	}
	data, err := base64.StdEncoding.DecodeString(value[comma+1:])
	if err != nil {
		return nil, err
	}
	return data, nil
}
