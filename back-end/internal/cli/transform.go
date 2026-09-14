package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fabricum/back-end/internal/editor"
	"fabricum/back-end/internal/processing"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const TransformReceiptSchemaVersion = 1

// TransformReceipt describes a completed noninteractive transform. The
// request is retained so consumers can associate each measurement with its
// project-neutral operations.
type TransformReceipt struct {
	SchemaVersion int                            `json:"schemaVersion"`
	Processor     string                         `json:"processor"`
	Source        string                         `json:"source"`
	Request       processing.TransformRequest    `json:"request"`
	Outputs       []processing.OutputMeasurement `json:"outputs"`
}

// RunTransform prepares, writes, and reports a noninteractive transform.
func RunTransform(ctx context.Context, request processing.TransformRequest, output io.Writer) error {
	request, err := absoluteTransformRequest(request, currentDirectory())
	if err != nil {
		return fmt.Errorf("transform: %w", err)
	}
	return runTransformRequest(ctx, normalizeTransformRequest(request), output)
}

// RunTransformFile reads a JSON transform request from path and writes its
// receipt to output. A path of "-" reads stdin; relative paths are resolved
// from the request file's directory.
func RunTransformFile(ctx context.Context, path string, output io.Writer) error {
	var reader io.Reader
	base := currentDirectory()
	var file *os.File
	if path == "-" {
		reader = os.Stdin
	} else {
		if path == "" {
			return errors.New("transform request path is required")
		}
		opened, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open transform request: %w", err)
		}
		file = opened
		defer file.Close()
		base, err = filepath.Abs(filepath.Dir(path))
		if err != nil {
			return fmt.Errorf("resolve transform request directory: %w", err)
		}
		reader = file
	}
	decoder := json.NewDecoder(io.LimitReader(reader, 1<<20))
	decoder.DisallowUnknownFields()
	var request processing.TransformRequest
	if err := decoder.Decode(&request); err != nil {
		return fmt.Errorf("decode transform request: %w", err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return err
	}
	request, err := absoluteTransformRequest(request, base)
	if err != nil {
		return fmt.Errorf("transform request: %w", err)
	}
	return runTransformRequest(ctx, normalizeTransformRequest(request), output)
}

func runTransformRequest(ctx context.Context, request processing.TransformRequest, output io.Writer) error {
	measurements, err := processing.Transform(ctx, request)
	if err != nil {
		return fmt.Errorf("transform: %w", err)
	}
	return writeTransformReceipt(output, TransformReceipt{
		SchemaVersion: TransformReceiptSchemaVersion,
		Processor:     "fabricum/" + editor.Version,
		Source:        request.Source,
		Request:       request,
		Outputs:       measurements,
	})
}

func writeTransformReceipt(output io.Writer, receipt TransformReceipt) error {
	if err := json.NewEncoder(output).Encode(receipt); err != nil {
		return fmt.Errorf("write transform receipt: %w", err)
	}
	return nil
}

func absoluteTransformRequest(request processing.TransformRequest, base string) (processing.TransformRequest, error) {
	var err error
	request.Source, err = absoluteTransformPath(request.Source, base)
	if err != nil {
		return processing.TransformRequest{}, err
	}
	request.EncoderDirectory, err = absoluteOptionalTransformPath(request.EncoderDirectory, base)
	if err != nil {
		return processing.TransformRequest{}, err
	}
	for index := range request.Outputs {
		request.Outputs[index].Path, err = absoluteTransformPath(request.Outputs[index].Path, base)
		if err != nil {
			return processing.TransformRequest{}, err
		}
		if pack := request.Outputs[index].Transform.Pack; pack != nil {
			for _, input := range []*processing.ChannelInput{&pack.Red, &pack.Green, &pack.Blue} {
				input.Source, err = absoluteOptionalTransformPath(input.Source, base)
				if err != nil {
					return processing.TransformRequest{}, err
				}
			}
		}
	}
	return request, nil
}

func absoluteTransformPath(path, base string) (string, error) {
	if path == "" {
		return "", nil
	}
	return absoluteOptionalTransformPath(path, base)
}

func absoluteOptionalTransformPath(path, base string) (string, error) {
	if path == "" || filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Abs(filepath.Join(base, path))
}

func currentDirectory() string {
	directory, err := os.Getwd()
	if err != nil {
		return "."
	}
	return directory
}

func normalizeTransformRequest(request processing.TransformRequest) processing.TransformRequest {
	request.Format = strings.ToLower(strings.TrimSpace(request.Format))
	if request.Format == "" {
		request.Format = "png"
	}
	return request
}
