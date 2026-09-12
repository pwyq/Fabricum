package fabricum

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	options, err := ParseConfig([]string{"-config", configPath, "-square-size", "6"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	config, err := configure(options)
	if err != nil {
		t.Fatal(err)
	}
	if config.sourcePath != source || config.squareOutput.width != 6 || config.wideOutput.width != 8 || config.squareOutput.path != filepath.Join(root, "deliveries", "arbitrary-square.webp") {
		t.Fatalf("unexpected config: %+v", config)
	}
	if _, err := NewHandler(options); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-help"}, {"-version"}} {
		if _, err := ParseConfig(args, io.Discard); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected clean help exit: %v", err)
		}
	}
	for _, text := range []string{`{"unknown":1}`, `{"source":"x"} {}`, `{"source":"x","wideWidth":7}`} {
		if err := os.WriteFile(configPath, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ParseConfig([]string{"-config", configPath}, io.Discard); err == nil {
			t.Fatalf("accepted invalid config: %s", text)
		}
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
	config, err := configure(options)
	if err != nil {
		t.Fatal(err)
	}
	app, err := newApplication(config)
	if err != nil {
		t.Fatal(err)
	}
	call := func(route string, data string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "http://localhost"+route, bytes.NewBufferString(data))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Unit-Art-Token", app.token)
		response := httptest.NewRecorder()
		app.handler().ServeHTTP(response, request)
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
	if receipt.SchemaVersion != 1 || receipt.Source != source || len(receipt.Outputs) != 2 || receipt.Processor != "fabricum/"+Version {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	assertImageDimensions(t, filepath.Join(root, "square.png"), 4, 4)
	assertImageDimensions(t, filepath.Join(root, "wide.png"), 4, 3)
	if err := exportCommand([]string{"node", "-e", "process.exit(7)"}, root)(source, receipt.Request, receipt.Outputs); err == nil {
		t.Fatal("hook failure must be returned")
	}
}

func TestInvalidPathsAndOutputCollisions(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.png")
	writeFixtureImage(t, source, 8, 8)
	for _, options := range []Config{
		{Source: filepath.Join(root, "missing.png"), SquareSize: 4, WideWidth: 4},
		{Source: source, SquareSize: 4, WideWidth: 4, SquareOutput: source},
		{Source: source, SquareSize: 4, WideWidth: 4, SquareOutput: "same.png", WideOutput: "same.webp"},
	} {
		if _, err := NewHandler(options); err == nil {
			t.Fatalf("accepted invalid options: %+v", options)
		}
	}
	if err := validateLocalAddress("0.0.0.0:4179"); err == nil {
		t.Fatal("accepted public listener")
	}
}

func TestParseConfigDefaultsToGUIAndCLIRequiresSource(t *testing.T) {
	gui, err := ParseConfig([]string{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if gui.Mode != ModeGUI {
		t.Fatalf("expected GUI default, got %q", gui.Mode)
	}
	if _, err := configure(gui); err != nil {
		t.Fatalf("GUI should start without a source: %v", err)
	}
	handler, err := NewHandler(gui)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://localhost/api/config", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("expected GUI config endpoint to start without a source, got %d", response.Code)
	}

	cli, err := ParseConfig([]string{"-mode", "cli"}, io.Discard)
	if err == nil {
		t.Fatal("CLI should require a source")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source.png")
	writeFixtureImage(t, source, 8, 8)
	cli, err = ParseConfig([]string{
		"-mode", "cli", "-source", source, "-square-size", "4", "-wide-width", "4",
	}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if cli.Mode != ModeCLI {
		t.Fatalf("expected CLI mode, got %q", cli.Mode)
	}
}
