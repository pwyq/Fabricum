package editor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGUILifecycleStopsAfterPageDisconnects(t *testing.T) {
	stopped := make(chan struct{})
	var stopOnce sync.Once
	lifecycle := newGUILifecycle(10*time.Millisecond, func() {
		stopOnce.Do(func() { close(stopped) })
	})
	server := httptest.NewServer(lifecycle)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("content type = %q", response.Header.Get("Content-Type"))
	}

	cancel()
	_ = response.Body.Close()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("GUI server remained alive after its page disconnected")
	}
}

func TestGUILifecycleStopsWithoutPageConnection(t *testing.T) {
	stopped := make(chan struct{})
	lifecycle := newGUILifecycle(10*time.Millisecond, func() { close(stopped) })
	lifecycle.requireConnectionWithin(10 * time.Millisecond)

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("GUI server remained alive without a connected page")
	}
}

func TestGUILifecycleSurvivesPageReload(t *testing.T) {
	stopped := make(chan struct{})
	lifecycle := newGUILifecycle(40*time.Millisecond, func() { close(stopped) })
	if !lifecycle.connect() {
		t.Fatal("first page did not connect")
	}
	lifecycle.disconnect()
	if !lifecycle.connect() {
		t.Fatal("reloaded page did not connect")
	}

	select {
	case <-stopped:
		t.Fatal("GUI server stopped during a page reload")
	case <-time.After(80 * time.Millisecond):
	}
	lifecycle.disconnect()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("GUI server remained alive after the reloaded page disconnected")
	}
}

func TestGUIServeReturnsAfterPageCloses(t *testing.T) {
	pageResult := make(chan error, 1)
	err := serve(Config{Mode: ModeGUI, Address: "127.0.0.1:0"}, func(address string) error {
		go func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, address+"api/lifecycle", nil)
			if err != nil {
				pageResult <- err
				return
			}
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				pageResult <- err
				return
			}
			cancel()
			pageResult <- response.Body.Close()
		}()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-pageResult; err != nil {
		t.Fatal(err)
	}
}

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
