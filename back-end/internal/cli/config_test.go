package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fabricum/back-end/internal/editor"
	"flag"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigPathsAndCLIOverrideOutsideRepository(t *testing.T) {
	root := t.TempDir()
	t.Chdir(t.TempDir())
	source := filepath.Join(root, "arbitrary.png")
	writeFixtureImage(t, source, 12, 8)
	configPath := filepath.Join(root, "settings.json")
	if err := os.WriteFile(configPath, []byte(`{"source":"arbitrary.png","squareSize":4,"wideWidth":8,"outputDirectory":"deliveries"}`), 0600); err != nil {
		t.Fatal(err)
	}
	options, err := ParseConfig([]string{"--config", configPath, "--square-size", "6"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if options.Source != source || options.SquareSize != 6 || options.WideWidth != 8 || options.OutputDirectory != filepath.Join(root, "deliveries") {
		t.Fatalf("unexpected config: %+v", options)
	}
	if _, err := editor.NewHandler(options); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--help"}, {"-h"}, {"--version"}} {
		if _, err := ParseConfig(args, io.Discard); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected clean help exit: %v", err)
		}
	}
	for _, text := range []string{`{"unknown":1}`, `{"mode":"gui"}`, `{"source":"x"} {}`, `{"source":"x","wideWidth":7}`} {
		if err := os.WriteFile(configPath, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ParseConfig([]string{"-config", configPath}, io.Discard); err == nil {
			t.Fatalf("accepted invalid config: %s", text)
		}
	}
}

func TestHelpShowsSimplifiedCommands(t *testing.T) {
	var output bytes.Buffer
	if _, err := ParseConfig([]string{"--help"}, &output); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected clean help exit: %v", err)
	}
	help := output.String()
	for _, expected := range []string{
		"fabricum --source path --output path",
		"fabricum -s path -o path",
		"-h, --help",
	} {
		if !strings.Contains(help, expected) {
			t.Fatalf("help is missing %q:\n%s", expected, help)
		}
	}
	if strings.Contains(help, "mode") {
		t.Fatalf("help still exposes mode selection:\n%s", help)
	}
}

func TestConfigSourceListAndExportCommand(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "sample.png")
	writeFixtureImage(t, source, 8, 8)
	command := filepath.Join(root, "receipt.mjs")
	if err := os.WriteFile(command, []byte(`import fs from 'node:fs';let data='';for await(const c of process.stdin)data+=c;fs.writeFileSync('receipt.json',data);`), 0600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "config.json")
	data, _ := json.Marshal(map[string]any{"squareSize": 4, "wideWidth": 4, "sources": []map[string]string{{"path": "sample.png", "squareOutput": "square.png", "wideOutput": "wide.png"}}, "exportCommand": []string{"node", command}})
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	options, err := ParseConfig([]string{"-config", configPath}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := editor.NewHandler(options)
	if err != nil {
		t.Fatal(err)
	}
	configRequest := httptest.NewRequest(http.MethodGet, "http://localhost/api/config", nil)
	configResponse := httptest.NewRecorder()
	handler.ServeHTTP(configResponse, configRequest)
	var initialConfig struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(configResponse.Body).Decode(&initialConfig); err != nil {
		t.Fatal(err)
	}
	call := func(route string, data string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "http://localhost"+route, bytes.NewBufferString(data))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Unit-Art-Token", initialConfig.Token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	selection, _ := json.Marshal(map[string]string{"path": source})
	if response := call("/api/source", string(selection)); response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	if response := call("/api/source", `{"path":"not-listed.png"}`); response.Code != 422 {
		t.Fatal(response.Code)
	}
	response := call("/api/export", `{"format":"png","square":{"x":0,"y":0,"width":8,"height":8},"wide":{"x":0,"y":1,"width":8,"height":6}}`)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	receiptBytes, err := os.ReadFile(filepath.Join(root, "receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	var receipt ExportReceipt
	if err := json.Unmarshal(receiptBytes, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.SchemaVersion != 1 || receipt.Source != source || len(receipt.Outputs) != 2 || receipt.Processor != "fabricum/"+editor.Version {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	assertImageDimensions(t, filepath.Join(root, "square.png"), 4, 4)
	assertImageDimensions(t, filepath.Join(root, "wide.png"), 4, 3)
}

func TestInvalidPathsAndOutputCollisions(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.png")
	writeFixtureImage(t, source, 8, 8)
	for _, options := range []editor.Config{
		{Source: filepath.Join(root, "missing.png"), SquareSize: 4, WideWidth: 4},
		{Source: source, SquareSize: 4, WideWidth: 4, SquareOutput: source},
		{Source: source, SquareSize: 4, WideWidth: 4, SquareOutput: "same.png", WideOutput: "same.webp"},
	} {
		if _, err := editor.NewHandler(options); err == nil {
			t.Fatalf("accepted invalid options: %+v", options)
		}
	}
	if _, err := ParseConfig([]string{"--address", "0.0.0.0:4179"}, io.Discard); err == nil {
		t.Fatal("accepted public listener")
	}
}

func TestParseConfigInfersGUIOrCLIFromPathFlags(t *testing.T) {
	gui, err := ParseConfig([]string{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if gui.Mode != editor.ModeGUI {
		t.Fatalf("expected GUI default, got %q", gui.Mode)
	}
	handler, err := editor.NewHandler(gui)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://localhost/api/config", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("expected GUI config endpoint to start without a source, got %d", response.Code)
	}

	cli, err := ParseConfig([]string{"--output", t.TempDir()}, io.Discard)
	if err == nil {
		t.Fatal("CLI output without a source should fail")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source.png")
	writeFixtureImage(t, source, 8, 8)
	for _, args := range [][]string{
		{"--source", source, "--output", filepath.Join(root, "long"), "--square-size", "4", "--wide-width", "4"},
		{"-s", source, "-o", filepath.Join(root, "short"), "--square-size", "4", "--wide-width", "4"},
	} {
		cli, err = ParseConfig(args, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		if cli.Mode != editor.ModeCLI {
			t.Fatalf("expected CLI mode, got %q", cli.Mode)
		}
		if cli.OutputDirectory != args[3] {
			t.Fatalf("expected output %q, got %q", args[3], cli.OutputDirectory)
		}
	}
	if _, err := ParseConfig([]string{"--mode", "cli", "--source", source}, io.Discard); err == nil {
		t.Fatal("obsolete --mode flag should be rejected")
	}
}

func writeFixtureImage(t *testing.T, path string, width, height int) {
	t.Helper()
	fixture := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			fixture.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 20), G: uint8(y * 30), B: uint8((x + y) * 10), A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, fixture); err != nil {
		_ = file.Close()
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
