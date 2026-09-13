package editor

import (
	"encoding/base64"
	"encoding/json"
	"fabricum/back-end/internal/processing"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type exportResponse struct {
	Outputs []processing.OutputMeasurement `json:"outputs"`
}

type previewResponse struct {
	Outputs  []processing.OutputMeasurement `json:"outputs"`
	DataURLs map[string]string              `json:"dataUrls"`
}

type previewCache struct {
	request processing.ExportRequest
	source  sourceRevision
	outputs []processing.ProcessedOutput
}

type sourceRevision struct {
	size             int64
	modifiedUnixNano int64
}

func (app *application) handlePreview(response http.ResponseWriter, request *http.Request) {
	input, ok := app.readExportRequest(response, request)
	if !ok {
		return
	}
	app.mutationMu.Lock()
	defer app.mutationMu.Unlock()
	if app.config.sourcePath == "" {
		writeError(response, http.StatusConflict, "select a source before previewing")
		return
	}
	outputs, err := app.previewOutputs(request, input)
	if err != nil {
		writeError(response, http.StatusUnprocessableEntity, err.Error())
		return
	}
	dataURLs := make(map[string]string, len(outputs))
	for _, output := range outputs {
		mimeType := "image/" + output.Measurement.Format
		dataURLs[output.Measurement.Role] = "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(output.Data)
	}
	writeJSON(response, http.StatusOK, previewResponse{Outputs: processing.OutputMeasurements(outputs), DataURLs: dataURLs})
}

func (app *application) handleExport(response http.ResponseWriter, request *http.Request) {
	input, ok := app.readExportRequest(response, request)
	if !ok {
		return
	}
	app.mutationMu.Lock()
	defer app.mutationMu.Unlock()
	if app.config.sourcePath == "" {
		writeError(response, http.StatusConflict, "select a source before exporting")
		return
	}
	outputs, err := app.previewOutputs(request, input)
	if err == nil {
		err = processing.WriteOutputs(outputs)
	}
	if err == nil && app.config.afterExport != nil {
		err = app.config.afterExport(app.config.sourcePath, input, processing.OutputMeasurements(outputs))
	}
	if err != nil {
		writeError(response, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(response, http.StatusOK, exportResponse{Outputs: processing.OutputMeasurements(outputs)})
}

func (app *application) previewOutputs(request *http.Request, input processing.ExportRequest) ([]processing.ProcessedOutput, error) {
	decoded, err := readSourceConfig(app.config.sourcePath)
	if err != nil {
		return nil, err
	}
	if err := validateSourceSize(decoded, app.config); err != nil {
		return nil, err
	}
	revision, err := revisionFor(app.config.sourcePath)
	if err != nil {
		return nil, err
	}
	if app.preview != nil && app.preview.request == input && app.preview.source == revision {
		return app.preview.outputs, nil
	}
	specs, err := selectedOutputSpecs(input.Role, app.config.squareOutput, app.config.wideOutput)
	if err != nil {
		return nil, err
	}
	outputs, err := processing.PrepareOutputs(request.Context(), app.config.sourcePath, input, specs, app.config.encoderDirectory)
	if err != nil {
		return nil, err
	}
	app.preview = &previewCache{request: input, source: revision, outputs: outputs}
	return outputs, nil
}

func selectedOutputSpecs(role string, square, wide processing.OutputSpec) ([]processing.OutputSpec, error) {
	switch role {
	case "":
		return []processing.OutputSpec{square, wide}, nil
	case "square":
		return []processing.OutputSpec{square}, nil
	case "wide":
		return []processing.OutputSpec{wide}, nil
	default:
		return nil, fmt.Errorf("unsupported output role %q", role)
	}
}

func (app *application) readExportRequest(response http.ResponseWriter, request *http.Request) (processing.ExportRequest, bool) {
	if request.Header.Get("X-Unit-Art-Token") != app.token {
		writeError(response, http.StatusForbidden, "invalid processor session token")
		return processing.ExportRequest{}, false
	}
	if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
		writeError(response, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return processing.ExportRequest{}, false
	}
	request.Body = http.MaxBytesReader(response, request.Body, 16<<10)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input processing.ExportRequest
	if err := decoder.Decode(&input); err != nil {
		writeError(response, http.StatusBadRequest, "invalid export request: "+err.Error())
		return processing.ExportRequest{}, false
	}
	if err := ensureJSONEnd(decoder); err != nil {
		writeError(response, http.StatusBadRequest, err.Error())
		return processing.ExportRequest{}, false
	}
	return input, true
}

func revisionFor(path string) (sourceRevision, error) {
	info, err := os.Stat(path)
	if err != nil {
		return sourceRevision{}, fmt.Errorf("measure source: %w", err)
	}
	return sourceRevision{size: info.Size(), modifiedUnixNano: info.ModTime().UnixNano()}, nil
}
