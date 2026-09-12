package fabricum

import (
	"errors"
	"log"
	"net/http"
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

// Serve runs the editor on a loopback address without opening a browser.
func Serve(options Config) error {
	if options.Address == "" {
		options.Address = "127.0.0.1:4179"
	}
	if err := validateLocalAddress(options.Address); err != nil {
		return err
	}
	handler, err := NewHandler(options)
	if err != nil {
		return err
	}
	server := &http.Server{Addr: options.Address, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("fabricum/%s ready at http://%s", Version, options.Address)
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// WriteFileAtomically replaces one file after synchronizing its temporary file.
// An export of multiple files is not a transaction.
func WriteFileAtomically(path string, data []byte) error { return writeFileAtomically(path, data) }
