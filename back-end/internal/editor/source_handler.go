package editor

import (
	"encoding/json"
	"mime"
	"net/http"
)

type sourceSelectionRequest struct {
	Path string `json:"path"`
}

func (app *application) handleSelectSource(response http.ResponseWriter, request *http.Request) {
	if request.Header.Get("X-Unit-Art-Token") != app.token {
		writeError(response, http.StatusForbidden, "invalid processor session token")
		return
	}
	if mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
		writeError(response, http.StatusUnsupportedMediaType, "source selection must be JSON")
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, 4<<10)
	var selection sourceSelectionRequest
	decoder := json.NewDecoder(request.Body)
	if err := decoder.Decode(&selection); err != nil {
		writeError(response, http.StatusBadRequest, "decode source selection: "+err.Error())
		return
	}
	if err := ensureJSONEnd(decoder); err != nil {
		writeError(response, http.StatusBadRequest, err.Error())
		return
	}
	app.mutationMu.Lock()
	defer app.mutationMu.Unlock()
	config, err := app.config.withSource(selection.Path)
	if err == nil {
		decoded, readErr := readSourceConfig(config.sourcePath)
		err = readErr
		if err == nil {
			err = validateSourceSize(decoded, config)
		}
	}
	if err != nil {
		writeError(response, http.StatusUnprocessableEntity, err.Error())
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
