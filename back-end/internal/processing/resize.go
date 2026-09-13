package processing

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
)

func validateCrop(bounds image.Rectangle, crop CropRect, spec OutputSpec) error {
	if crop.Width <= 0 || crop.Height <= 0 {
		return errors.New("dimensions must be positive")
	}
	if crop.X < 0 || crop.Y < 0 || crop.Width > bounds.Dx() || crop.Height > bounds.Dy() || crop.X > bounds.Dx()-crop.Width || crop.Y > bounds.Dy()-crop.Height {
		return fmt.Errorf("rectangle %+v exceeds %dx%d source", crop, bounds.Dx(), bounds.Dy())
	}
	if crop.Width*spec.Height != crop.Height*spec.Width {
		return fmt.Errorf("rectangle must use %d:%d aspect ratio", spec.Width, spec.Height)
	}
	if crop.Width < spec.Width || crop.Height < spec.Height {
		return fmt.Errorf("rectangle must be at least %dx%d to avoid upscaling", spec.Width, spec.Height)
	}
	return nil
}

func resizeCrop(source image.Image, crop CropRect, width, height int) *image.NRGBA {
	result := image.NewNRGBA(image.Rect(0, 0, width, height))
	for destinationY := 0; destinationY < height; destinationY++ {
		sourceY := float64(crop.Y) + (float64(destinationY)+0.5)*float64(crop.Height)/float64(height) - 0.5
		for destinationX := 0; destinationX < width; destinationX++ {
			sourceX := float64(crop.X) + (float64(destinationX)+0.5)*float64(crop.Width)/float64(width) - 0.5
			result.SetNRGBA(destinationX, destinationY, bilinearSample(source, sourceX, sourceY, crop))
		}
	}
	return result
}

func bilinearSample(source image.Image, sourceX, sourceY float64, crop CropRect) color.NRGBA {
	minX, minY := crop.X, crop.Y
	maxX, maxY := crop.X+crop.Width-1, crop.Y+crop.Height-1
	x0 := clamp(int(math.Floor(sourceX)), minX, maxX)
	y0 := clamp(int(math.Floor(sourceY)), minY, maxY)
	x1, y1 := clamp(x0+1, minX, maxX), clamp(y0+1, minY, maxY)
	xWeight, yWeight := sourceX-float64(x0), sourceY-float64(y0)
	if x0 == x1 {
		xWeight = 0
	}
	if y0 == y1 {
		yWeight = 0
	}
	top := interpolateColor(nrgbaAt(source, x0, y0), nrgbaAt(source, x1, y0), xWeight)
	bottom := interpolateColor(nrgbaAt(source, x0, y1), nrgbaAt(source, x1, y1), xWeight)
	return interpolateColor(top, bottom, yWeight)
}

func nrgbaAt(source image.Image, x, y int) color.NRGBA {
	return color.NRGBAModel.Convert(source.At(x+source.Bounds().Min.X, y+source.Bounds().Min.Y)).(color.NRGBA)
}

func interpolateColor(left, right color.NRGBA, weight float64) color.NRGBA {
	return color.NRGBA{
		R: interpolateChannel(left.R, right.R, weight), G: interpolateChannel(left.G, right.G, weight),
		B: interpolateChannel(left.B, right.B, weight), A: interpolateChannel(left.A, right.A, weight),
	}
}

func interpolateChannel(left, right uint8, weight float64) uint8 {
	return uint8(math.Round(float64(left)*(1-weight) + float64(right)*weight))
}

func clamp(value, minimum, maximum int) int {
	return min(max(value, minimum), maximum)
}
