package fabricum

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
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
		return fmt.Errorf("listen on %s: %w", options.Address, err)
	}
	address := listener.Addr().String()
	server := &http.Server{Addr: address, Handler: app.handler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second}
	browserAddress := browserURL(address)
	log.Printf("fabricum/%s ready at %s", Version, browserAddress)
	if config.mode == ModeGUI {
		if err := openBrowser(browserAddress); err != nil {
			log.Printf("could not open the browser automatically; open %s manually: %v", browserAddress, err)
		}
	}
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func browserURL(address string) string {
	return (&url.URL{Scheme: "http", Host: address, Path: "/"}).String()
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
	root := filepath.Dir(filepath.Dir(executable))
	if _, err := os.Stat(filepath.Join(root, "node_modules", "sharp")); err != nil {
		return ""
	}
	return root
}

// WriteFileAtomically replaces one file after synchronizing its temporary file.
// An export of multiple files is not a transaction.
func WriteFileAtomically(path string, data []byte) error { return writeFileAtomically(path, data) }
