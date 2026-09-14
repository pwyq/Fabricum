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
)

const TextureReceiptSchemaVersion = 1

// TextureReceipt describes a completed loose KTX2 texture set. The request is
// retained so integrations can associate each measurement with its transforms.
type TextureReceipt struct {
	SchemaVersion         int                            `json:"schemaVersion"`
	Processor             string                         `json:"processor"`
	Request               processing.TextureSetRequest   `json:"request"`
	Outputs               []processing.OutputMeasurement `json:"outputs"`
	NativeEncoderVersions map[string]string              `json:"nativeEncoderVersions"`
}

// RunTextureSet prepares, writes, and reports a loose KTX2 texture set.
func RunTextureSet(ctx context.Context, request processing.TextureSetRequest, output io.Writer) error {
	request, err := absoluteTextureSetRequest(request, currentDirectory())
	if err != nil {
		return fmt.Errorf("texture set: %w", err)
	}
	return runTextureSetRequest(ctx, request, output)
}

// RunTextureSetFile reads a JSON texture-set request from path and writes its
// versioned receipt to output. A path of "-" reads stdin.
func RunTextureSetFile(ctx context.Context, path string, output io.Writer) error {
	var reader io.Reader
	base := currentDirectory()
	var file *os.File
	if path == "-" {
		reader = os.Stdin
	} else {
		if path == "" {
			return errors.New("texture set request path is required")
		}
		opened, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open texture set request: %w", err)
		}
		file = opened
		defer file.Close()
		base, err = filepath.Abs(filepath.Dir(path))
		if err != nil {
			return fmt.Errorf("resolve texture set request directory: %w", err)
		}
		reader = file
	}
	decoder := json.NewDecoder(io.LimitReader(reader, 1<<20))
	decoder.DisallowUnknownFields()
	var request processing.TextureSetRequest
	if err := decoder.Decode(&request); err != nil {
		return fmt.Errorf("decode texture set request: %w", err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return err
	}
	request, err := absoluteTextureSetRequest(request, base)
	if err != nil {
		return fmt.Errorf("texture set request: %w", err)
	}
	return runTextureSetRequest(ctx, request, output)
}

func runTextureSetRequest(ctx context.Context, request processing.TextureSetRequest, output io.Writer) error {
	measurements, err := processing.BuildTextureSet(ctx, request)
	if err != nil {
		return fmt.Errorf("texture set: %w", err)
	}
	if err := json.NewEncoder(output).Encode(TextureReceipt{
		SchemaVersion:         TextureReceiptSchemaVersion,
		Processor:             "fabricum/" + editor.Version,
		Request:               request,
		Outputs:               measurements,
		NativeEncoderVersions: processing.EncoderVersionsForFormat("ktx2"),
	}); err != nil {
		return fmt.Errorf("write texture set receipt: %w", err)
	}
	return nil
}

func absoluteTextureSetRequest(request processing.TextureSetRequest, base string) (processing.TextureSetRequest, error) {
	var err error
	request.EncoderDirectory, err = absoluteOptionalTransformPath(request.EncoderDirectory, base)
	if err != nil {
		return processing.TextureSetRequest{}, err
	}
	for index := range request.Outputs {
		request.Outputs[index].Source, err = absoluteTransformPath(request.Outputs[index].Source, base)
		if err != nil {
			return processing.TextureSetRequest{}, err
		}
		request.Outputs[index].Path, err = absoluteTransformPath(request.Outputs[index].Path, base)
		if err != nil {
			return processing.TextureSetRequest{}, err
		}
		if pack := request.Outputs[index].Transform.Pack; pack != nil {
			for _, input := range []*processing.ChannelInput{&pack.Red, &pack.Green, &pack.Blue} {
				input.Source, err = absoluteOptionalTransformPath(input.Source, base)
				if err != nil {
					return processing.TextureSetRequest{}, err
				}
			}
		}
	}
	return request, nil
}
