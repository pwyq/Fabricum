package editor

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRejectsForeignHostAndOriginBeforeExposingToken(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	for _, sample := range []struct {
		host, origin string
		status       int
	}{
		{"localhost:4179", "", 200},
		{"127.0.0.1:4179", "http://127.0.0.1:4179", 200},
		{"[::1]:4179", "", 200},
		{"foreign.invalid:4179", "", 403},
		{"localhost:4179", "http://foreign.invalid", 403},
		{"localhost:4179", "null", 403},
	} {
		request := httptest.NewRequest("GET", "http://"+sample.host+"/api/config", nil)
		request.Header.Set("Origin", sample.origin)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != sample.status {
			t.Fatalf("%+v: got %d", sample, response.Code)
		}
	}
}
