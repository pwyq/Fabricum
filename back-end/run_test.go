package fabricum

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGUIReopensExistingFabricumInstance(t *testing.T) {
	existing := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/config" {
			http.NotFound(response, request)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"processor":"fabricum/0.1.0"}`))
	}))
	defer existing.Close()

	address := strings.TrimPrefix(existing.URL, "http://")
	var opened string
	err := serve(Config{Mode: ModeGUI, Address: address}, func(url string) error {
		opened = url
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if opened != existing.URL+"/" {
		t.Fatalf("opened %q, want %q", opened, existing.URL+"/")
	}
}

func TestGUIDoesNotOpenUnrelatedListener(t *testing.T) {
	existing := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"processor":"something-else"}`))
	}))
	defer existing.Close()

	address := strings.TrimPrefix(existing.URL, "http://")
	opened := false
	err := serve(Config{Mode: ModeGUI, Address: address}, func(string) error {
		opened = true
		return nil
	})
	if err == nil {
		t.Fatal("expected the occupied address to remain an error")
	}
	if opened {
		t.Fatal("opened a browser for an unrelated listener")
	}
}
