package processing

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	maxTransformOutputs = 1024
	maxTransformPath    = 4096
	maxTransformPixels  = 64 << 20
)

func (request TransformRequest) effectiveConstraints() SourceConstraints {
	if request.SourceConstraints != nil {
		return *request.SourceConstraints
	}
	return request.Constraints
}

func validateTransformRequest(request TransformRequest) error {
	if request.Source == "" || len(request.Source) > maxTransformPath {
		return errors.New("source path must contain between 1 and 4096 bytes")
	}
	if len(request.Outputs) == 0 {
		return errors.New("at least one transform output is required")
	}
	if len(request.Outputs) > maxTransformOutputs {
		return fmt.Errorf("at most %d transform outputs are allowed", maxTransformOutputs)
	}
	if err := validateEncodingOptions(ExportRequest{Format: request.Format, PNGMode: request.PNGMode, Quality: request.Quality, Lossless: request.Lossless}); err != nil {
		return err
	}
	constraints := request.effectiveConstraints()
	if constraints.Width < 0 || constraints.Height < 0 || (constraints.Width == 0) != (constraints.Height == 0) {
		return errors.New("source width and height must both be positive or both be zero")
	}
	if constraints.Format != "" {
		if _, ok := normalizeImageFormat(constraints.Format); !ok {
			return fmt.Errorf("unsupported source format %q", constraints.Format)
		}
	}
	for index, output := range request.Outputs {
		if output.Path == "" || len(output.Path) > maxTransformPath {
			return fmt.Errorf("output %d path must contain between 1 and 4096 bytes", index+1)
		}
		if err := validateImageTransform(output.Transform); err != nil {
			return fmt.Errorf("output %s: %w", outputRole(output, index), err)
		}
	}
	return nil
}

func validateTransformPaths(request TransformRequest) error {
	source, err := filepath.Abs(request.Source)
	if err != nil {
		return fmt.Errorf("resolve source path: %w", err)
	}
	paths := make([]string, 0, len(request.Outputs))
	for index, output := range request.Outputs {
		path := OutputPath(output.Path, request.Format)
		absolute, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve %s output path: %w", outputRole(output, index), err)
		}
		if sameTransformPath(source, absolute) {
			return errors.New("source and transform output paths must be distinct")
		}
		for _, existing := range paths {
			if sameTransformPath(existing, absolute) {
				return errors.New("transform output paths must be distinct")
			}
		}
		paths = append(paths, absolute)
	}
	return nil
}

func sameTransformPath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtimeWindows() {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func runtimeWindows() bool {
	return filepath.Separator == '\\'
}

func outputRole(output TransformOutputSpec, index int) string {
	if output.Role != "" {
		return output.Role
	}
	return fmt.Sprintf("output-%d", index+1)
}

func validateImageTransform(transform ImageTransform) error {
	if transform.Crop != nil {
		if transform.Crop.Width <= 0 || transform.Crop.Height <= 0 || transform.Crop.X < 0 || transform.Crop.Y < 0 {
			return errors.New("crop coordinates and dimensions must be non-negative, with positive dimensions")
		}
	}
	if transform.Resize != nil {
		if err := validateResizeSpec(*transform.Resize); err != nil {
			return err
		}
	}
	if transform.Padding != nil {
		padding := transform.Padding
		if padding.Top < 0 || padding.Right < 0 || padding.Bottom < 0 || padding.Left < 0 {
			return errors.New("padding values cannot be negative")
		}
	}
	if transform.Channel != "" && !validChannel(transform.Channel) {
		return fmt.Errorf("unsupported channel %q", transform.Channel)
	}
	if transform.Pack != nil {
		for _, channel := range []struct {
			name  string
			input ChannelInput
		}{
			{name: "red", input: transform.Pack.Red},
			{name: "green", input: transform.Pack.Green},
			{name: "blue", input: transform.Pack.Blue},
		} {
			if err := validateChannelInput(channel.name, channel.input); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateResizeSpec(resize ResizeSpec) error {
	if resize.Width < 1 || resize.Height < 1 || resize.Width > 8192 || resize.Height > 8192 {
		return errors.New("resize dimensions must be between 1 and 8192")
	}
	if int64(resize.Width)*int64(resize.Height) > maxTransformPixels {
		return errors.New("resize dimensions exceed 67108864 pixels")
	}
	fit := strings.ToLower(strings.TrimSpace(resize.Fit))
	if fit != "" && fit != "fill" && fit != "contain" {
		return fmt.Errorf("resize fit must be fill or contain, got %q", resize.Fit)
	}
	if _, ok := normalizeFilter(resize.Filter); !ok {
		return fmt.Errorf("unsupported resize filter %q", resize.Filter)
	}
	return nil
}

func validateChannelInput(name string, input ChannelInput) error {
	if input.Constant != nil {
		if input.Source != "" || input.Channel != "" {
			return fmt.Errorf("%s channel constant cannot include a source or channel", name)
		}
		return nil
	}
	if input.Channel != "" && !validChannel(input.Channel) {
		return fmt.Errorf("unsupported %s channel %q", name, input.Channel)
	}
	if input.Source != "" && len(input.Source) > maxTransformPath {
		return fmt.Errorf("%s channel source path is too long", name)
	}
	return nil
}
