package processing

import (
	"image"
	"image/color"
	"math"
	"strings"
)

type resizeWeight struct {
	index  int
	weight float64
}

type resamplePixel struct {
	r, g, b, a float64
}

func normalizeFilter(filter string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(filter)) {
	case "":
		return "lanczos3", true
	case "nearest":
		return "nearest", true
	case "bilinear", "linear":
		return "bilinear", true
	case "lanczos3", "lanczos":
		return "lanczos3", true
	default:
		return "", false
	}
}

func effectiveFilter(filter string) string {
	normalized, _ := normalizeFilter(filter)
	return normalized
}

func resizeImage(source image.Image, sourceRect image.Rectangle, width, height int, filter string) *image.NRGBA {
	if sourceRect.Dx() == width && sourceRect.Dy() == height {
		return copyImage(source, sourceRect)
	}
	horizontal := resizeWeights(sourceRect.Dx(), width, filter)
	vertical := resizeWeights(sourceRect.Dy(), height, filter)
	temporary := make([]resamplePixel, width*sourceRect.Dy())
	for sourceY := 0; sourceY < sourceRect.Dy(); sourceY++ {
		for destinationX := 0; destinationX < width; destinationX++ {
			var pixel resamplePixel
			for _, sample := range horizontal[destinationX] {
				value := nrgbaAtAbsolute(source, sourceRect.Min.X+sample.index, sourceRect.Min.Y+sourceY)
				contribution := premultiply(value)
				pixel.r += contribution.r * sample.weight
				pixel.g += contribution.g * sample.weight
				pixel.b += contribution.b * sample.weight
				pixel.a += contribution.a * sample.weight
			}
			temporary[sourceY*width+destinationX] = pixel
		}
	}
	result := image.NewNRGBA(image.Rect(0, 0, width, height))
	for destinationY := 0; destinationY < height; destinationY++ {
		for destinationX := 0; destinationX < width; destinationX++ {
			var pixel resamplePixel
			for _, sample := range vertical[destinationY] {
				value := temporary[sample.index*width+destinationX]
				pixel.r += value.r * sample.weight
				pixel.g += value.g * sample.weight
				pixel.b += value.b * sample.weight
				pixel.a += value.a * sample.weight
			}
			result.SetNRGBA(destinationX, destinationY, unpremultiply(pixel))
		}
	}
	return result
}

func resizeWeights(sourceSize, destinationSize int, filter string) [][]resizeWeight {
	normalized := effectiveFilter(filter)
	weights := make([][]resizeWeight, destinationSize)
	if normalized == "nearest" {
		for destination := range weights {
			position := (float64(destination)+0.5)*float64(sourceSize)/float64(destinationSize) - 0.5
			index := clamp(int(math.Floor(position+0.5)), 0, sourceSize-1)
			weights[destination] = []resizeWeight{{index: index, weight: 1}}
		}
		return weights
	}
	support := 1.0
	if normalized == "lanczos3" {
		support = 3
	}
	scale := min(1.0, float64(destinationSize)/float64(sourceSize))
	radius := support / scale
	for destination := range weights {
		position := (float64(destination)+0.5)*float64(sourceSize)/float64(destinationSize) - 0.5
		start := int(math.Ceil(position - radius))
		end := int(math.Floor(position + radius))
		values := make([]resizeWeight, 0, end-start+1)
		for index := start; index <= end; index++ {
			distance := math.Abs(position - float64(index))
			weight := filterWeight(normalized, distance, scale)
			if weight == 0 {
				continue
			}
			values = append(values, resizeWeight{index: clamp(index, 0, sourceSize-1), weight: weight})
		}
		if len(values) == 0 {
			index := clamp(int(math.Round(position)), 0, sourceSize-1)
			weights[destination] = []resizeWeight{{index: index, weight: 1}}
			continue
		}
		var total float64
		for _, value := range values {
			total += value.weight
		}
		if math.Abs(total) < 1e-12 {
			index := clamp(int(math.Round(position)), 0, sourceSize-1)
			weights[destination] = []resizeWeight{{index: index, weight: 1}}
			continue
		}
		for index := range values {
			values[index].weight /= total
		}
		weights[destination] = values
	}
	return weights
}

func filterWeight(filter string, distance, scale float64) float64 {
	if filter == "bilinear" {
		distance *= scale
		return scale * max(0, 1-distance)
	}
	distance *= scale
	return scale * lanczos3(distance)
}

func lanczos3(value float64) float64 {
	value = math.Abs(value)
	if value >= 3 {
		return 0
	}
	if value == 0 {
		return 1
	}
	return (math.Sin(math.Pi*value) / (math.Pi * value)) * (math.Sin(math.Pi*value/3) / (math.Pi * value / 3))
}

func premultiply(pixel color.NRGBA) resamplePixel {
	alpha := float64(pixel.A) / 255
	return resamplePixel{r: float64(pixel.R) * alpha, g: float64(pixel.G) * alpha, b: float64(pixel.B) * alpha, a: alpha}
}

func unpremultiply(pixel resamplePixel) color.NRGBA {
	alpha := max(0.0, min(1.0, pixel.a))
	if alpha == 0 {
		return color.NRGBA{}
	}
	return color.NRGBA{
		R: clampByte(pixel.r / alpha), G: clampByte(pixel.g / alpha), B: clampByte(pixel.b / alpha),
		A: clampByte(alpha * 255),
	}
}

func clampByte(value float64) uint8 {
	return uint8(max(0.0, min(255.0, math.Round(value))))
}
