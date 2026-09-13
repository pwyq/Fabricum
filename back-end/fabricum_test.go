package fabricum_test

import (
	"net/http"
	"testing"

	fabricum "fabricum/back-end"
)

func TestPublicHostInterfaceRemainsUsable(t *testing.T) {
	options := fabricum.Config{
		Mode: fabricum.ModeGUI,
		AfterExport: func(string, fabricum.ExportRequest, []fabricum.OutputMeasurement) error {
			return nil
		},
	}
	handler, err := fabricum.NewHandler(options)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := handler.(http.Handler); !ok {
		t.Fatal("NewHandler did not return an HTTP handler")
	}
	if fabricum.Version == "" {
		t.Fatal("Version is empty")
	}
}
