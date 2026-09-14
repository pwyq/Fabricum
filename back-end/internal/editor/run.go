package editor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	guiConnectTimeout  = 15 * time.Second
	guiDisconnectGrace = 1500 * time.Millisecond
)

// NewHandler constructs the local editor and processing API.
// Hosts must only serve this handler on a loopback listener.
func NewHandler(options Config) (http.Handler, error) {
	config, err := configure(options)
	if err != nil {
		return nil, err
	}
	app, err := newApplication(config)
	if err != nil {
		return nil, err
	}
	return app.handler(), nil
}

// Serve runs the editor on a loopback address. GUI mode opens the local URL;
// CLI mode prints it without starting a browser.
func Serve(options Config) error {
	return serve(options, openBrowser)
}

func serve(options Config, browserOpener func(string) error) error {
	if options.Address == "" {
		options.Address = "127.0.0.1:4179"
	}
	if err := validateLocalAddress(options.Address); err != nil {
		return err
	}
	if options.EncoderDirectory == "" {
		options.EncoderDirectory = executableEncoderDirectory()
	}
	config, err := configure(options)
	if err != nil {
		return err
	}
	app, err := newApplication(config)
	if err != nil {
		return err
	}
	defer app.cleanup()
	listener, err := net.Listen("tcp", options.Address)
	if err != nil {
		browserAddress := browserURL(options.Address)
		if config.mode == ModeGUI && isFabricumServer(browserAddress) {
			if openErr := browserOpener(browserAddress); openErr != nil {
				return fmt.Errorf("listen on %s: %w; another Fabricum instance is ready at %s, but the browser could not be opened: %v", options.Address, err, browserAddress, openErr)
			}
			log.Printf("fabricum/%s already ready at %s", Version, browserAddress)
			return nil
		}
		return fmt.Errorf("listen on %s: %w", options.Address, err)
	}
	address := listener.Addr().String()
	var lifecycle *guiLifecycle
	var server *http.Server
	if config.mode == ModeGUI {
		lifecycle = newGUILifecycle(guiDisconnectGrace, func() {
			stopServer(server)
		})
		app.lifecycle = lifecycle
	}
	server = &http.Server{Addr: address, Handler: app.handler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second}
	browserAddress := browserURL(address)
	log.Printf("fabricum/%s ready at %s", Version, browserAddress)
	if config.mode == ModeGUI {
		serveResult := make(chan error, 1)
		go func() { serveResult <- server.Serve(listener) }()
		if err := browserOpener(browserAddress); err != nil {
			stopServer(server)
			<-serveResult
			return fmt.Errorf("open browser: %w", err)
		}
		lifecycle.requireConnectionWithin(guiConnectTimeout)
		err = <-serveResult
	} else {
		err = server.Serve(listener)
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func stopServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		_ = server.Close()
	}
}

func browserURL(address string) string {
	return (&url.URL{Scheme: "http", Host: address, Path: "/"}).String()
}

func isFabricumServer(address string) bool {
	client := &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Get(address + "api/config")
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false
	}
	var identity struct {
		Processor string `json:"processor"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&identity); err != nil {
		return false
	}
	return strings.HasPrefix(identity.Processor, "fabricum/")
}

func openBrowser(address string) error {
	var command string
	var arguments []string
	switch runtime.GOOS {
	case "windows":
		command = "rundll32"
		arguments = []string{"url.dll,FileProtocolHandler", address}
	case "darwin":
		command = "open"
		arguments = []string{address}
	default:
		command = "xdg-open"
		arguments = []string{address}
	}
	process := exec.Command(command, arguments...)
	if err := process.Start(); err != nil {
		return err
	}
	go func() { _ = process.Wait() }()
	return nil
}

func executableEncoderDirectory() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	directory := filepath.Join(filepath.Dir(executable), "codecs")
	if nativeToolsPresent(directory) {
		return directory
	}
	return ""
}

func nativeToolsPresent(directory string) bool {
	for _, name := range []string{"cwebp", "avifenc"} {
		path := filepath.Join(directory, name)
		if runtime.GOOS == "windows" {
			path += ".exe"
		}
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}
