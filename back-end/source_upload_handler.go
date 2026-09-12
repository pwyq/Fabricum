package fabricum

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const maxSourceUploadBytes = 32 << 20

func (app *application) handleImport(response http.ResponseWriter, request *http.Request) {
	data, decoded, _, ok := app.readImageUpload(response, request, "import")
	if !ok {
		return
	}
	app.mutationMu.Lock()
	defer app.mutationMu.Unlock()
	if app.config.sourcePath == "" {
		writeError(response, http.StatusConflict, "select a source before importing")
		return
	}
	if err := validateSourceSize(decoded, app.config); err != nil {
		writeError(response, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := writeFileAtomically(app.config.sourcePath, data); err != nil {
		writeError(response, http.StatusInternalServerError, "write imported source: "+err.Error())
		return
	}
	app.preview = nil
	config, err := app.clientConfig()
	if err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(response, http.StatusOK, config)
}

func (app *application) handleSourceUpload(response http.ResponseWriter, request *http.Request) {
	data, decoded, format, ok := app.readImageUpload(response, request, "source upload")
	if !ok {
		return
	}
	app.mutationMu.Lock()
	defer app.mutationMu.Unlock()
	if app.config.sourcePath != "" {
		writeError(response, http.StatusConflict, "a source is already selected; use import to replace it")
		return
	}
	configWithoutSourceList := app.config
	if app.config.sources != nil {
		sources, err := app.config.sources()
		if err != nil {
			writeError(response, http.StatusInternalServerError, err.Error())
			return
		}
		if len(sources) > 0 {
			writeError(response, http.StatusConflict, "source upload is unavailable with a configured source list")
			return
		}
		configWithoutSourceList.sources = nil
	}
	if err := validateSourceSize(decoded, app.config); err != nil {
		writeError(response, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := app.ensureUploadDirectory(); err != nil {
		writeError(response, http.StatusInternalServerError, "create temporary source directory: "+err.Error())
		return
	}
	sourcePath := filepath.Join(app.uploadDir, sourceUploadFilename(request.Header.Get("X-Fabricum-Source-Name"), format))
	config, err := configWithoutSourceList.withSource(sourcePath)
	if err != nil {
		writeError(response, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := writeFileAtomically(sourcePath, data); err != nil {
		writeError(response, http.StatusInternalServerError, "write uploaded source: "+err.Error())
		return
	}
	app.config = config
	app.preview = nil
	clientConfig, err := app.clientConfig()
	if err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(response, http.StatusOK, clientConfig)
}

func (app *application) readImageUpload(response http.ResponseWriter, request *http.Request, action string) ([]byte, image.Config, string, bool) {
	if request.Header.Get("X-Unit-Art-Token") != app.token {
		writeError(response, http.StatusForbidden, "invalid processor session token")
		return nil, image.Config{}, "", false
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || formatForMediaType(mediaType) == "" {
		writeError(response, http.StatusUnsupportedMediaType, action+" must be a PNG, JPEG, or GIF file")
		return nil, image.Config{}, "", false
	}
	request.Body = http.MaxBytesReader(response, request.Body, maxSourceUploadBytes)
	data, err := io.ReadAll(request.Body)
	if err != nil {
		writeError(response, http.StatusBadRequest, "read "+action+": "+err.Error())
		return nil, image.Config{}, "", false
	}
	decoded, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || mediaTypeForFormat(format) != mediaType {
		writeError(response, http.StatusBadRequest, action+" is not a valid PNG, JPEG, or GIF")
		return nil, image.Config{}, "", false
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		writeError(response, http.StatusBadRequest, action+" is not a complete image")
		return nil, image.Config{}, "", false
	}
	return data, decoded, format, true
}

func (app *application) ensureUploadDirectory() error {
	if app.uploadDir != "" {
		return nil
	}
	directory, err := os.MkdirTemp("", "fabricum-gui-")
	if err != nil {
		return err
	}
	app.uploadDir = directory
	return nil
}

func (app *application) cleanup() {
	if app.uploadDir != "" {
		_ = os.RemoveAll(app.uploadDir)
	}
}

func sourceUploadFilename(rawName, format string) string {
	name := filepath.Base(strings.ReplaceAll(strings.TrimSpace(rawName), "\\", "/"))
	if name == "" || name == "." || name == ".." {
		return "source." + format
	}
	name = strings.Map(func(character rune) rune {
		if character < 0x20 || strings.ContainsRune(`<>:"/\\|?*`, character) {
			return '_'
		}
		return character
	}, name)
	if name == "" || name == "." || name == ".." {
		return "source." + format
	}
	extension := strings.ToLower(filepath.Ext(name))
	if extension != ".png" && extension != ".jpg" && extension != ".jpeg" && extension != ".gif" {
		name = strings.TrimSuffix(name, filepath.Ext(name)) + "." + format
	}
	if len(name) > 128 {
		name = name[:128]
	}
	return name
}

func mediaTypeForFormat(format string) string {
	switch format {
	case "png":
		return "image/png"
	case "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	default:
		return ""
	}
}

func formatForMediaType(mediaType string) string {
	switch mediaType {
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpeg"
	case "image/gif":
		return "gif"
	default:
		return ""
	}
}

func sourceMediaType(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	_, format, err := image.DecodeConfig(file)
	if err != nil {
		return "", fmt.Errorf("decode source configuration: %w", err)
	}
	mediaType := mediaTypeForFormat(format)
	if mediaType == "" {
		return "", fmt.Errorf("unsupported source format %q", format)
	}
	return mediaType, nil
}
