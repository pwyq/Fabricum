package processing

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// SVGPathError bounds centerline positions in this path's local user units.
// Multiply by the composed transform's largest scale and viewport pixel scale
// for a display-space bound, accounting separately for viewBox changes.
// It excludes stroke-width rounding and is not a raster/stroke-edge comparison.
type SVGPathError struct {
	PathIndex       int     `json:"pathIndex"`
	MaxDeviation    float64 `json:"maxDeviation"`
	PreservedReason string  `json:"preservedReason,omitempty"`
}

func roundSVGNumber(token string, value float64, options SVGOptimizationOptions) (string, float64) {
	if options.Precision != nil {
		scale := math.Pow10(*options.Precision)
		if math.Abs(value) < 1e15/scale {
			value = math.Round(value*scale) / scale
			token = strconv.FormatFloat(value, 'f', -1, 64)
		}
	}
	if options.SelectiveRounding {
		for _, places := range []int{3, 2} {
			digits, terminal := svgDecimalTail(token)
			if digits == places && (terminal == '1' || terminal == '9') && math.Abs(value) < 1e12 {
				scale := math.Pow10(places - 1)
				value = math.Round(value*scale) / scale
				token = strconv.FormatFloat(value, 'f', -1, 64)
			}
		}
	}
	return token, value
}

// Inspect decimal spelling before float64 conversion can discard terminal
// digits. Normalize trailing zeroes and exponent notation without changing the
// number, so .110 is eligible but .11000000000000000001 is not.
func svgDecimalTail(token string) (int, byte) {
	mantissa, exponent, _ := strings.Cut(strings.ToLower(token), "e")
	shift := 0
	if exponent != "" {
		var err error
		shift, err = strconv.Atoi(exponent)
		if err != nil || shift > 10000 || shift < -10000 {
			return 0, 0
		}
	}
	_, fraction, _ := strings.Cut(mantissa, ".")
	digits := strings.ReplaceAll(strings.TrimLeft(mantissa, "+-"), ".", "")
	trimmed := strings.TrimRight(digits, "0")
	if trimmed == "" {
		return 0, 0
	}
	return len(fraction) - shift - (len(digits) - len(trimmed)), trimmed[len(trimmed)-1]
}

func optimizeSVGPaths(root *svgNode, options SVGOptimizationOptions) ([]SVGPathError, error) {
	var reports []SVGPathError
	index := 0
	var visit func(*svgNode) error
	visit = func(n *svgNode) error {
		if n.name.Local == "path" && n.name.Space == svgNamespace && n.attribute("d") != "" {
			index++
			segments, err := parseSVGPath(n.attribute("d"))
			if err != nil {
				return fmt.Errorf("SVG path %d: %w", index, err)
			}
			// Arc geometry is sensitive to radii correction and axis rotation.
			// Keep the entire path exact, including preceding relative segments.
			arc := false
			for _, segment := range segments {
				arc = arc || segment.command|32 == 'a'
			}
			var state svgPathDelta
			for i, segment := range segments {
				deltas := make([]float64, len(segment.values))
				if !arc && (options.Precision != nil || options.SelectiveRounding) {
					for j, original := range segment.values {
						token, rounded := roundSVGNumber(segment.numbers[j], original, options)
						deltas[j] = rounded - original
						segments[i].numbers[j] = token
					}
					state.segment(segment.command, deltas)
				}
			}
			if math.IsNaN(state.bound) || math.IsInf(state.bound, 0) {
				return fmt.Errorf("SVG path %d: non-finite geometry bound", index)
			}
			serialized := writeSVGPath(segments)
			if _, err := parseSVGPath(serialized); err != nil {
				return fmt.Errorf("invalid optimized SVG path %d: %w", index, err)
			}
			setSVGAttribute(n, "d", serialized)
			if options.Precision != nil || options.SelectiveRounding {
				report := SVGPathError{PathIndex: index, MaxDeviation: state.bound}
				if arc {
					report.PreservedReason = "Arc path retained at original precision."
				}
				reports = append(reports, report)
			}
		}
		for _, child := range n.children {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	err := visit(root)
	return reports, err
}

type svgPoint struct{ x, y float64 }

func (p svgPoint) add(q svgPoint) svgPoint     { return svgPoint{p.x + q.x, p.y + q.y} }
func (p svgPoint) reflect(q svgPoint) svgPoint { return svgPoint{2*p.x - q.x, 2*p.y - q.y} }

// Track error vectors directly, so each relative endpoint accumulates previous
// error rather than being treated as an independent rounded coordinate. Bezier
// convex hulls bound the entire curve by its control/end point errors; smooth
// commands reflect the prior control error and closepath resets to subpath start.
type svgPathDelta struct {
	current, start, control svgPoint
	previous                byte
	bound                   float64
}

func (s *svgPathDelta) include(p svgPoint) {
	s.bound = math.Max(s.bound, math.Hypot(p.x, p.y))
}

func (s *svgPathDelta) segment(command byte, delta []float64) {
	lower := command | 32
	origin := svgPoint{}
	if command >= 'a' && command <= 'z' {
		origin = s.current
	}
	point := func(i int) svgPoint { return origin.add(svgPoint{delta[i], delta[i+1]}) }
	s.include(s.current)
	switch lower {
	case 'm', 'l':
		s.current = point(0)
		if lower == 'm' {
			s.start = s.current
		}
	case 'h':
		s.current.x = origin.x + delta[0]
	case 'v':
		s.current.y = origin.y + delta[0]
	case 'c':
		s.include(point(0))
		s.control = point(2)
		s.include(s.control)
		s.current = point(4)
	case 's':
		control := s.current
		if s.previous == 'c' || s.previous == 's' {
			control = s.current.reflect(s.control)
		}
		s.include(control)
		s.control = point(0)
		s.include(s.control)
		s.current = point(2)
	case 'q':
		s.control = point(0)
		s.include(s.control)
		s.current = point(2)
	case 't':
		control := s.current
		if s.previous == 'q' || s.previous == 't' {
			control = s.current.reflect(s.control)
		}
		s.control = control
		s.include(control)
		s.current = point(0)
	case 'z':
		s.current = s.start
	}
	s.include(s.current)
	s.previous = lower
}
