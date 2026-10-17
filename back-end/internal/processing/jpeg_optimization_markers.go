package processing

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type jpegPart struct {
	marker byte
	raw    []byte
}

type optimizationJPEG struct {
	parts       []jpegPart
	progressive bool
}

func jpegMetadata(marker byte) bool { return marker >= 0xe0 && marker <= 0xef || marker == 0xfe }

// Parse the complete stream, including markers between progressive scans.
// Reject containers whose signatures or absolute offsets a rewrite invalidates.
func parseOptimizationJPEG(data []byte) (optimizationJPEG, error) {
	var result optimizationJPEG
	if len(data) > maxInspectionFileBytes || len(data) < 4 || !bytes.Equal(data[:2], []byte{0xff, 0xd8}) {
		return result, fmt.Errorf("invalid JPEG signature or file size")
	}
	frame, scans := false, 0
	for pos := 2; pos < len(data); {
		start := pos
		if data[pos] != 0xff {
			return result, fmt.Errorf("expected JPEG marker at byte %d", pos)
		}
		for pos < len(data) && data[pos] == 0xff {
			pos++
		}
		if pos == len(data) {
			break
		}
		marker := data[pos]
		pos++
		if marker == 0xd9 {
			if !frame || scans == 0 || pos != len(data) {
				return result, fmt.Errorf("missing image data or trailing JPEG data")
			}
			result.parts = append(result.parts, jpegPart{marker, data[start:pos]})
			return result, nil
		}
		if pos+2 > len(data) {
			break
		}
		length := int(binary.BigEndian.Uint16(data[pos:]))
		if length < 2 || length > len(data)-pos {
			return result, fmt.Errorf("invalid JPEG marker length")
		}
		body := data[pos+2 : pos+length]
		switch {
		case marker == 0xc0 || marker == 0xc2:
			if frame || len(body) < 6 || body[0] != 8 || (body[5] != 1 && body[5] != 3) || len(body) != 6+3*int(body[5]) {
				return result, fmt.Errorf("only single-frame 8-bit grayscale and three-component DCT JPEGs are supported")
			}
			width, height := int(binary.BigEndian.Uint16(body[3:])), int(binary.BigEndian.Uint16(body[1:]))
			if width == 0 || height == 0 || int64(width)*int64(height) > maxInspectionPixels {
				return result, fmt.Errorf("JPEG dimensions exceed the %d pixel limit", maxInspectionPixels)
			}
			frame, result.progressive = true, marker == 0xc2
		case jpegMetadata(marker):
			if marker == 0xeb || marker == 0xe2 && bytes.HasPrefix(body, []byte("MPF\x00")) {
				return result, fmt.Errorf("JPEG APP11 signatures/JUMBF and multi-picture MPF data are unsupported")
			}
		case marker == 0xda:
			scans++
			if !frame || scans > 256 {
				return result, fmt.Errorf("invalid JPEG scan order or scan count")
			}
		case marker == 0xdb || marker == 0xc4 || marker == 0xdd:
		default:
			return result, fmt.Errorf("unsupported JPEG marker 0x%02x", marker)
		}
		pos += length
		result.parts = append(result.parts, jpegPart{marker, data[start:pos]})
		if marker == 0xda {
			start = pos
			for pos < len(data) {
				if data[pos] != 0xff {
					pos++
					continue
				}
				next := pos + 1
				for next < len(data) && data[next] == 0xff {
					next++
				}
				if next == len(data) || data[next] != 0 && (data[next] < 0xd0 || data[next] > 0xd7) {
					break
				}
				pos = next + 1
			}
			result.parts = append(result.parts, jpegPart{0, data[start:pos]})
		}
	}
	return result, fmt.Errorf("truncated JPEG without end marker")
}

// Copy every APPn/COM verbatim, preserving their relative order. jpegtran may
// otherwise synthesize JFIF/Adobe markers or discard JFIF thumbnails.
func rebuildOptimizedJPEG(source, candidate optimizationJPEG) []byte {
	output := []byte{0xff, 0xd8}
	for _, part := range source.parts {
		if jpegMetadata(part.marker) {
			output = append(output, part.raw...)
		}
	}
	for _, part := range candidate.parts {
		if !jpegMetadata(part.marker) {
			output = append(output, part.raw...)
		}
	}
	return output
}

func sameJPEGMetadata(a, b optimizationJPEG) bool {
	collect := func(jpeg optimizationJPEG) []byte {
		var data []byte
		for _, part := range jpeg.parts {
			if jpegMetadata(part.marker) {
				data = append(data, part.raw...)
			}
		}
		return data
	}
	return bytes.Equal(collect(a), collect(b))
}
