package processing

import (
	"fmt"
	"image"
	"image/color"
	"strings"
)

func applyChannelTransform(source image.Image, transform ImageTransform) image.Image {
	if transform.Channel != "" {
		result := image.NewGray(image.Rect(0, 0, source.Bounds().Dx(), source.Bounds().Dy()))
		for y := 0; y < result.Bounds().Dy(); y++ {
			for x := 0; x < result.Bounds().Dx(); x++ {
				result.SetGray(x, y, colorGray(channelValue(nrgbaAtAbsolute(source, source.Bounds().Min.X+x, source.Bounds().Min.Y+y), transform.Channel)))
			}
		}
		return result
	}
	result := copyImage(source, source.Bounds())
	if transform.Grayscale {
		for y := 0; y < result.Bounds().Dy(); y++ {
			for x := 0; x < result.Bounds().Dx(); x++ {
				pixel := result.NRGBAAt(x, y)
				gray := grayscaleValue(pixel)
				result.SetNRGBA(x, y, color.NRGBA{R: gray, G: gray, B: gray, A: pixel.A})
			}
		}
	}
	if transform.RemoveAlpha {
		for y := 0; y < result.Bounds().Dy(); y++ {
			for x := 0; x < result.Bounds().Dx(); x++ {
				pixel := result.NRGBAAt(x, y)
				pixel.A = 255
				result.SetNRGBA(x, y, pixel)
			}
		}
	}
	return result
}

func packChannels(primarySpatial image.Image, transform ImageTransform) (image.Image, error) {
	pack := transform.Pack
	result := image.NewNRGBA(image.Rect(0, 0, primarySpatial.Bounds().Dx(), primarySpatial.Bounds().Dy()))
	for y := 0; y < result.Bounds().Dy(); y++ {
		for x := 0; x < result.Bounds().Dx(); x++ {
			result.SetNRGBA(x, y, color.NRGBA{A: 255})
		}
	}
	inputs := []struct {
		name  string
		input ChannelInput
		set   func(*color.NRGBA, uint8)
	}{
		{name: "red", input: pack.Red, set: func(pixel *color.NRGBA, value uint8) { pixel.R = value }},
		{name: "green", input: pack.Green, set: func(pixel *color.NRGBA, value uint8) { pixel.G = value }},
		{name: "blue", input: pack.Blue, set: func(pixel *color.NRGBA, value uint8) { pixel.B = value }},
	}
	for _, entry := range inputs {
		if entry.input.Constant != nil {
			for y := 0; y < result.Bounds().Dy(); y++ {
				for x := 0; x < result.Bounds().Dx(); x++ {
					pixel := result.NRGBAAt(x, y)
					entry.set(&pixel, *entry.input.Constant)
					result.SetNRGBA(x, y, pixel)
				}
			}
			continue
		}
		channelSource := primarySpatial
		if entry.input.Source != "" {
			decoded, _, err := readTransformImage(entry.input.Source, SourceConstraints{})
			if err != nil {
				return nil, fmt.Errorf("%s channel: %w", entry.name, err)
			}
			channelSource, _, err = applySpatialTransform(decoded, transform)
			if err != nil {
				return nil, fmt.Errorf("%s channel: %w", entry.name, err)
			}
		}
		if channelSource.Bounds().Dx() != result.Bounds().Dx() || channelSource.Bounds().Dy() != result.Bounds().Dy() {
			return nil, fmt.Errorf("%s channel dimensions are %dx%d, want %dx%d", entry.name, channelSource.Bounds().Dx(), channelSource.Bounds().Dy(), result.Bounds().Dx(), result.Bounds().Dy())
		}
		channel := entry.input.Channel
		if channel == "" {
			channel = "gray"
		}
		for y := 0; y < result.Bounds().Dy(); y++ {
			for x := 0; x < result.Bounds().Dx(); x++ {
				pixel := result.NRGBAAt(x, y)
				entry.set(&pixel, channelValue(nrgbaAtAbsolute(channelSource, channelSource.Bounds().Min.X+x, channelSource.Bounds().Min.Y+y), channel))
				result.SetNRGBA(x, y, pixel)
			}
		}
	}
	return result, nil
}

func validChannel(channel string) bool {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "r", "red", "g", "green", "b", "blue", "a", "alpha", "gray", "grey", "luma", "luminance":
		return true
	default:
		return false
	}
}

func channelValue(pixel color.NRGBA, channel string) uint8 {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "r", "red":
		return pixel.R
	case "g", "green":
		return pixel.G
	case "b", "blue":
		return pixel.B
	case "a", "alpha":
		return pixel.A
	default:
		return grayscaleValue(pixel)
	}
}

func grayscaleValue(pixel color.NRGBA) uint8 {
	return uint8((299*uint32(pixel.R) + 587*uint32(pixel.G) + 114*uint32(pixel.B) + 500) / 1000)
}

func colorGray(value uint8) color.Gray {
	return color.Gray{Y: value}
}
