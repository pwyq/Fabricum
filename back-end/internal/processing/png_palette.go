package processing

import (
	"image"
	"image/color"
	"image/png"
	"sort"
)

const indexedPNGColorLimit = 256
const maximumExactPalettePixels = 1 << 20

type paletteColor struct {
	value            color.NRGBA
	premulR, premulG uint8
	premulB          uint8
	count            int
}

type colorBox struct {
	colors []paletteColor
	count  int
}

type paletteHistogram struct {
	red, green, blue, alpha int64
	count                   int
}

func encodeIndexedPNG(source image.Image, compression png.CompressionLevel) ([]byte, error) {
	palette := buildPalette(source, indexedPNGColorLimit)
	indexed := image.NewPaletted(source.Bounds(), palette)
	cache := make(map[uint32]uint8)
	paletteValues := make([]paletteColor, len(palette))
	for index, value := range palette {
		paletteValues[index] = newPaletteColor(color.NRGBAModel.Convert(value).(color.NRGBA), 0)
	}
	bounds := source.Bounds()
	exactMapping := bounds.Dx()*bounds.Dy() <= maximumExactPalettePixels
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := canonicalPaletteColor(color.NRGBAModel.Convert(source.At(x, y)).(color.NRGBA))
			key := paletteHistogramKey(pixel, exactMapping)
			index, ok := cache[key]
			if !ok {
				index = nearestPaletteIndex(newPaletteColor(pixel, 0), paletteValues)
				cache[key] = index
			}
			indexed.SetColorIndex(x, y, index)
		}
	}
	return encodePNG(indexed, compression)
}

func buildPalette(source image.Image, limit int) color.Palette {
	histogram := make(map[uint32]paletteHistogram)
	hasTransparent := false
	bounds := source.Bounds()
	exactHistogram := bounds.Dx()*bounds.Dy() <= maximumExactPalettePixels
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			value := canonicalPaletteColor(color.NRGBAModel.Convert(source.At(x, y)).(color.NRGBA))
			if value.A == 0 {
				hasTransparent = true
				continue
			}
			key := paletteHistogramKey(value, exactHistogram)
			entry := histogram[key]
			entry.red += int64(value.R)
			entry.green += int64(value.G)
			entry.blue += int64(value.B)
			entry.alpha += int64(value.A)
			entry.count++
			histogram[key] = entry
		}
	}
	colors := make([]paletteColor, 0, len(histogram))
	for _, entry := range histogram {
		count := int64(entry.count)
		colors = append(colors, newPaletteColor(color.NRGBA{
			R: uint8(entry.red / count), G: uint8(entry.green / count),
			B: uint8(entry.blue / count), A: uint8(entry.alpha / count),
		}, entry.count))
	}
	sort.Slice(colors, func(i, j int) bool { return paletteColorKey(colors[i].value) < paletteColorKey(colors[j].value) })
	colorLimit := limit
	palette := make(color.Palette, 0, limit)
	if hasTransparent {
		palette = append(palette, color.NRGBA{})
		colorLimit--
	}
	if len(colors) == 0 {
		return palette
	}
	boxes := []colorBox{newColorBox(colors)}
	for len(boxes) < colorLimit {
		index := boxToSplit(boxes)
		if index < 0 {
			break
		}
		left, right := splitColorBox(boxes[index])
		boxes[index] = left
		boxes = append(boxes, right)
	}
	for _, box := range boxes {
		palette = append(palette, averageBoxColor(box))
	}
	return palette
}

func newPaletteColor(value color.NRGBA, count int) paletteColor {
	alpha := uint32(value.A)
	return paletteColor{
		value: value, count: count,
		premulR: uint8((uint32(value.R)*alpha + 127) / 255),
		premulG: uint8((uint32(value.G)*alpha + 127) / 255),
		premulB: uint8((uint32(value.B)*alpha + 127) / 255),
	}
}

func canonicalPaletteColor(value color.NRGBA) color.NRGBA {
	if value.A == 0 {
		return color.NRGBA{}
	}
	return value
}

func paletteColorKey(value color.NRGBA) uint32 {
	return uint32(value.R)<<24 | uint32(value.G)<<16 | uint32(value.B)<<8 | uint32(value.A)
}

func paletteHistogramKey(value color.NRGBA, exact bool) uint32 {
	if exact {
		return paletteColorKey(value)
	}
	return uint32(value.R>>3)<<15 | uint32(value.G>>3)<<10 | uint32(value.B>>3)<<5 | uint32(value.A>>3)
}

func newColorBox(colors []paletteColor) colorBox {
	box := colorBox{colors: colors}
	for _, value := range colors {
		box.count += value.count
	}
	return box
}

func boxToSplit(boxes []colorBox) int {
	selected, selectedScore := -1, -1
	for index, box := range boxes {
		_, colorRange := widestColorRange(box.colors)
		score := colorRange * box.count
		if len(box.colors) > 1 && score > selectedScore {
			selected, selectedScore = index, score
		}
	}
	return selected
}

func splitColorBox(box colorBox) (colorBox, colorBox) {
	axis, _ := widestColorRange(box.colors)
	sort.SliceStable(box.colors, func(i, j int) bool {
		left, right := paletteComponent(box.colors[i], axis), paletteComponent(box.colors[j], axis)
		if left == right {
			return paletteColorKey(box.colors[i].value) < paletteColorKey(box.colors[j].value)
		}
		return left < right
	})
	half, count, split := box.count/2, 0, 1
	for index := 0; index < len(box.colors)-1; index++ {
		count += box.colors[index].count
		split = index + 1
		if count >= half {
			break
		}
	}
	return newColorBox(box.colors[:split]), newColorBox(box.colors[split:])
}

func widestColorRange(colors []paletteColor) (int, int) {
	minimum := [4]int{255, 255, 255, 255}
	maximum := [4]int{}
	for _, value := range colors {
		for axis := range 4 {
			component := paletteComponent(value, axis)
			minimum[axis] = min(minimum[axis], component)
			maximum[axis] = max(maximum[axis], component)
		}
	}
	selected, selectedRange := 0, -1
	for axis := range 4 {
		valueRange := maximum[axis] - minimum[axis]
		if axis == 3 {
			valueRange *= 2
		}
		if valueRange > selectedRange {
			selected, selectedRange = axis, valueRange
		}
	}
	return selected, selectedRange
}

func paletteComponent(value paletteColor, axis int) int {
	switch axis {
	case 0:
		return int(value.premulR)
	case 1:
		return int(value.premulG)
	case 2:
		return int(value.premulB)
	default:
		return int(value.value.A)
	}
}

func averageBoxColor(box colorBox) color.NRGBA {
	var premulR, premulG, premulB, alpha int64
	for _, value := range box.colors {
		count := int64(value.count)
		premulR += int64(value.premulR) * count
		premulG += int64(value.premulG) * count
		premulB += int64(value.premulB) * count
		alpha += int64(value.value.A) * count
	}
	averageAlpha := alpha / int64(box.count)
	if averageAlpha == 0 {
		return color.NRGBA{}
	}
	straight := func(total int64) uint8 {
		averagePremul := total / int64(box.count)
		return uint8(min(255, int((averagePremul*255+averageAlpha/2)/averageAlpha)))
	}
	return color.NRGBA{R: straight(premulR), G: straight(premulG), B: straight(premulB), A: uint8(averageAlpha)}
}

func nearestPaletteIndex(value paletteColor, palette []paletteColor) uint8 {
	selected, selectedDistance := 0, int64(^uint64(0)>>1)
	for index, candidate := range palette {
		difference := func(left, right uint8) int64 { return int64(left) - int64(right) }
		dr := difference(value.premulR, candidate.premulR)
		dg := difference(value.premulG, candidate.premulG)
		db := difference(value.premulB, candidate.premulB)
		da := difference(value.value.A, candidate.value.A)
		distance := dr*dr + dg*dg + db*db + 2*da*da
		if distance < selectedDistance {
			selected, selectedDistance = index, distance
		}
	}
	return uint8(selected)
}
