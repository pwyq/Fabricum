package processing

import (
	"errors"
	"fmt"
	"image"
	"strings"
)

func renderTransform(primary image.Image, transform ImageTransform) (image.Image, string, error) {
	spatial, filter, err := applySpatialTransform(primary, transform)
	if err != nil {
		return nil, "", err
	}
	if transform.Pack != nil {
		packed, err := packChannels(spatial, transform)
		return packed, filter, err
	}
	return applyChannelTransform(spatial, transform), filter, nil
}

func applySpatialTransform(source image.Image, transform ImageTransform) (image.Image, string, error) {
	current := source
	if transform.Crop != nil {
		rect, err := cropRectangle(source.Bounds(), *transform.Crop)
		if err != nil {
			return nil, "", err
		}
		current = copyImage(source, rect)
	}
	filter := ""
	if transform.Resize != nil {
		resize := *transform.Resize
		filter = effectiveFilter(resize.Filter)
		fit := strings.ToLower(strings.TrimSpace(resize.Fit))
		if fit == "" {
			fit = "fill"
		}
		if fit == "fill" {
			current = resizeImage(current, current.Bounds(), resize.Width, resize.Height, filter)
		} else {
			current = resizeContain(current, resize.Width, resize.Height, filter)
		}
	}
	if transform.Padding != nil {
		var err error
		current, err = padTransparent(current, *transform.Padding)
		if err != nil {
			return nil, filter, err
		}
	}
	if err := validateTransformDimensions(current.Bounds()); err != nil {
		return nil, filter, err
	}
	return current, filter, nil
}

func validateTransformDimensions(bounds image.Rectangle) error {
	if bounds.Dx() < 1 || bounds.Dy() < 1 || bounds.Dx() > 8192 || bounds.Dy() > 8192 || int64(bounds.Dx())*int64(bounds.Dy()) > maxTransformPixels {
		return errors.New("output dimensions exceed 8192 pixels per side or 67108864 pixels")
	}
	return nil
}

func cropRectangle(bounds image.Rectangle, crop CropRect) (image.Rectangle, error) {
	if crop.Width <= 0 || crop.Height <= 0 {
		return image.Rectangle{}, errors.New("crop dimensions must be positive")
	}
	if crop.X < 0 || crop.Y < 0 || crop.Width > bounds.Dx() || crop.Height > bounds.Dy() || crop.X > bounds.Dx()-crop.Width || crop.Y > bounds.Dy()-crop.Height {
		return image.Rectangle{}, fmt.Errorf("crop rectangle %+v exceeds %dx%d source", crop, bounds.Dx(), bounds.Dy())
	}
	return image.Rect(bounds.Min.X+crop.X, bounds.Min.Y+crop.Y, bounds.Min.X+crop.X+crop.Width, bounds.Min.Y+crop.Y+crop.Height), nil
}

func copyImage(source image.Image, rect image.Rectangle) *image.NRGBA {
	result := image.NewNRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	for y := 0; y < rect.Dy(); y++ {
		for x := 0; x < rect.Dx(); x++ {
			result.SetNRGBA(x, y, nrgbaAtAbsolute(source, rect.Min.X+x, rect.Min.Y+y))
		}
	}
	return result
}

func resizeContain(source image.Image, width, height int, filter string) image.Image {
	sourceWidth, sourceHeight := source.Bounds().Dx(), source.Bounds().Dy()
	var scaledWidth, scaledHeight int
	if int64(width)*int64(sourceHeight) <= int64(height)*int64(sourceWidth) {
		scaledWidth = width
		scaledHeight = roundedRatio(sourceHeight, width, sourceWidth)
	} else {
		scaledWidth = roundedRatio(sourceWidth, height, sourceHeight)
		scaledHeight = height
	}
	scaledWidth = max(1, min(width, scaledWidth))
	scaledHeight = max(1, min(height, scaledHeight))
	resized := resizeImage(source, source.Bounds(), scaledWidth, scaledHeight, filter)
	result := image.NewNRGBA(image.Rect(0, 0, width, height))
	offsetX, offsetY := (width-scaledWidth)/2, (height-scaledHeight)/2
	for y := 0; y < scaledHeight; y++ {
		for x := 0; x < scaledWidth; x++ {
			result.SetNRGBA(offsetX+x, offsetY+y, nrgbaAt(resized, x, y))
		}
	}
	return result
}

func roundedRatio(value, numerator, denominator int) int {
	return int((int64(value)*int64(numerator) + int64(denominator)/2) / int64(denominator))
}

func padTransparent(source image.Image, padding PaddingSpec) (image.Image, error) {
	width := int64(source.Bounds().Dx()) + int64(padding.Left) + int64(padding.Right)
	height := int64(source.Bounds().Dy()) + int64(padding.Top) + int64(padding.Bottom)
	if width < 1 || height < 1 || width > 8192 || height > 8192 || width*height > maxTransformPixels {
		return nil, errors.New("padded dimensions exceed output limits")
	}
	result := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)))
	for y := 0; y < source.Bounds().Dy(); y++ {
		for x := 0; x < source.Bounds().Dx(); x++ {
			result.SetNRGBA(padding.Left+x, padding.Top+y, nrgbaAtAbsolute(source, source.Bounds().Min.X+x, source.Bounds().Min.Y+y))
		}
	}
	return result, nil
}
