package editor

import (
	"io"
	"net/http"
	"sync"
	"time"
)

type guiLifecycle struct {
	mu            sync.Mutex
	clients       int
	everConnected bool
	generation    uint64
	stopping      bool
	grace         time.Duration
	stop          func()
}

func newGUILifecycle(grace time.Duration, stop func()) *guiLifecycle {
	return &guiLifecycle{grace: grace, stop: stop}
}

func (lifecycle *guiLifecycle) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	controller := http.NewResponseController(response)
	_ = controller.SetWriteDeadline(time.Time{})
	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(http.StatusOK)
	if _, err := io.WriteString(response, ": connected\n\n"); err != nil {
		return
	}
	if err := controller.Flush(); err != nil || !lifecycle.connect() {
		return
	}
	defer lifecycle.disconnect()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-request.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := io.WriteString(response, ": keepalive\n\n"); err != nil {
				return
			}
			if err := controller.Flush(); err != nil {
				return
			}
		}
	}
}

func (lifecycle *guiLifecycle) connect() bool {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if lifecycle.stopping {
		return false
	}
	lifecycle.clients++
	lifecycle.everConnected = true
	lifecycle.generation++
	return true
}

func (lifecycle *guiLifecycle) disconnect() {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	lifecycle.clients--
	if lifecycle.clients != 0 {
		return
	}
	lifecycle.generation++
	generation := lifecycle.generation
	time.AfterFunc(lifecycle.grace, func() {
		lifecycle.stopWhenIdle(generation)
	})
}

func (lifecycle *guiLifecycle) requireConnectionWithin(timeout time.Duration) {
	time.AfterFunc(timeout, func() {
		lifecycle.mu.Lock()
		if lifecycle.everConnected || lifecycle.stopping {
			lifecycle.mu.Unlock()
			return
		}
		lifecycle.stopping = true
		lifecycle.mu.Unlock()
		lifecycle.stop()
	})
}

func (lifecycle *guiLifecycle) stopWhenIdle(generation uint64) {
	lifecycle.mu.Lock()
	if lifecycle.clients != 0 || lifecycle.generation != generation || lifecycle.stopping {
		lifecycle.mu.Unlock()
		return
	}
	lifecycle.stopping = true
	lifecycle.mu.Unlock()
	lifecycle.stop()
}
