package fabricum

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	frontend "fabricum/front-end"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type application struct {
	config     processorConfig
	token      string
	static     http.Handler
	mutationMu sync.Mutex
	preview    *previewCache
	uploadDir  string
}

type clientConfig struct {
	Processor string               `json:"processor"`
	Token     string               `json:"token"`
	Source    *clientSource        `json:"source,omitempty"`
	Sources   []string             `json:"sources"`
	Outputs   []clientOutputConfig `json:"outputs"`
}

type clientSource struct {
	URL    string `json:"url"`
	Path   string `json:"path"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Bytes  int64  `json:"bytes"`
}

type clientOutputConfig struct {
	Role   string `json:"role"`
	Path   string `json:"path"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func newApplication(config processorConfig) (*application, error) {
	if config.sourcePath != "" {
		decoded, err := readSourceConfig(config.sourcePath)
		if err != nil {
			return nil, err
		}
		if err := validateSourceSize(decoded, config); err != nil {
			return nil, err
		}
	}
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("create session token: %w", err)
	}
	staticRoot, err := fs.Sub(frontend.Files, "static")
	if err != nil {
		return nil, err
	}
	return &application{config: config, token: hex.EncodeToString(tokenBytes), static: http.FileServer(http.FS(staticRoot))}, nil
}

func (app *application) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/config", app.handleConfig)
	mux.HandleFunc("POST /api/source", app.handleSelectSource)
	mux.HandleFunc("POST /api/source-upload", app.handleSourceUpload)
	mux.HandleFunc("POST /api/export", app.handleExport)
	mux.HandleFunc("POST /api/preview", app.handlePreview)
	mux.HandleFunc("POST /api/import", app.handleImport)
	mux.HandleFunc("GET /source", app.handleSource)
	mux.Handle("/", app.static)
	return securityHeaders(mux)
}

func (app *application) handleConfig(response http.ResponseWriter, _ *http.Request) {
	app.mutationMu.Lock()
	defer app.mutationMu.Unlock()
	config, err := app.clientConfig()
	if err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(response, http.StatusOK, config)
}

func (app *application) clientConfig() (clientConfig, error) {
	config := clientConfig{
		Processor: "fabricum/" + Version,
		Token:     app.token,
		Sources:   []string{},
		Outputs:   []clientOutputConfig{},
	}
	if app.config.sources != nil {
		sources, err := app.config.sources()
		if err != nil {
			return clientConfig{}, err
		}
		for _, source := range sources {
			config.Sources = append(config.Sources, source.Path)
		}
	}
	if app.config.sourcePath == "" {
		return config, nil
	}
	source, err := readSourceConfig(app.config.sourcePath)
	if err != nil {
		return clientConfig{}, err
	}
	if err := validateSourceSize(source, app.config); err != nil {
		return clientConfig{}, err
	}
	info, err := os.Stat(app.config.sourcePath)
	if err != nil {
		return clientConfig{}, fmt.Errorf("measure source: %w", err)
	}
	config.Source = &clientSource{
		URL: fmt.Sprintf("/source?v=%d-%d", info.ModTime().UnixNano(), info.Size()), Path: filepath.ToSlash(app.config.sourcePath),
		Width: source.Width, Height: source.Height, Bytes: info.Size(),
	}
	config.Outputs = []clientOutputConfig{
		clientOutput(app.config.squareOutput),
		clientOutput(app.config.wideOutput),
	}
	return config, nil
}

func clientOutput(spec outputSpec) clientOutputConfig {
	return clientOutputConfig{Role: spec.role, Path: filepath.ToSlash(spec.path), Width: spec.width, Height: spec.height}
}

func (app *application) handleSource(response http.ResponseWriter, request *http.Request) {
	app.mutationMu.Lock()
	sourcePath := app.config.sourcePath
	app.mutationMu.Unlock()
	if sourcePath == "" {
		writeError(response, http.StatusNotFound, "source is not configured")
		return
	}
	if mediaType, err := sourceMediaType(sourcePath); err == nil {
		response.Header().Set("Content-Type", mediaType)
	}
	response.Header().Set("Cache-Control", "no-store")
	http.ServeFile(response, request, sourcePath)
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err == io.EOF {
		return nil
	} else if err != nil {
		return fmt.Errorf("invalid trailing JSON: %w", err)
	}
	return fmt.Errorf("request must contain one JSON value")
}

func writeError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, map[string]string{"error": message})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !localRequest(request) {
			writeError(response, http.StatusForbidden, "request must use the local processor origin")
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		response.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; script-src 'self'; style-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'")
		response.Header().Set("Referrer-Policy", "no-referrer")
		response.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(response, request)
	})
}
