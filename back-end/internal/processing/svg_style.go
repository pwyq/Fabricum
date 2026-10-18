package processing

import (
	"regexp"
	"strconv"
	"strings"
)

var svgStyleNumber = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)
var svgStyleLength = regexp.MustCompile(`^([+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?)(?:px|pt|pc|mm|cm|in|em|ex|%)?$`)
var svgHexColor = regexp.MustCompile(`^#(?:[0-9a-f]{3}|[0-9a-f]{4}|[0-9a-f]{6}|[0-9a-f]{8})$`)
var svgLocalPaint = regexp.MustCompile(`^url\(\s*(?:#[A-Za-z_][A-Za-z0-9_.:-]*|'#[A-Za-z_][A-Za-z0-9_.:-]*'|"#[A-Za-z_][A-Za-z0-9_.:-]*")\s*\)$`)

func svgSimpleColor(value string) bool {
	value = strings.ToLower(value)
	return svgHexColor.MatchString(value) || svgContains(strings.Fields("currentcolor transparent black white red green blue gray grey yellow cyan magenta orange purple pink brown lime navy teal aqua silver maroon olive fuchsia rebeccapurple"), value)
}

// Accept only well-understood declaration values. Invalid CSS declarations are
// ignored by renderers; replacing them with attributes could instead erase an
// earlier valid declaration or presentation attribute. Complex values therefore
// keep the original inline style and the conservative optimizer profile.
func svgSimpleStyleValue(name, value string) bool {
	if svgContains(strings.Fields("inherit initial unset revert"), value) {
		return true
	}
	oneOf := func(choices string) bool { return svgContains(strings.Fields(choices), value) }
	switch name {
	case "fill", "stroke":
		return value == "none" || svgSimpleColor(value) || svgLocalPaint.MatchString(value)
	case "color", "stop-color":
		return svgSimpleColor(value)
	case "fill-rule", "clip-rule":
		return oneOf("nonzero evenodd")
	case "display":
		return oneOf("inline block none")
	case "visibility":
		return oneOf("visible hidden collapse")
	case "overflow":
		return oneOf("visible hidden scroll auto clip")
	case "vector-effect":
		return oneOf("none non-scaling-stroke")
	case "enable-background":
		return value == "accumulate"
	case "stroke-linecap":
		return oneOf("butt round square")
	case "stroke-linejoin":
		return oneOf("miter miter-clip round bevel arcs")
	case "clip-path", "mask", "filter":
		return value == "none" || svgLocalPaint.MatchString(value)
	case "paint-order":
		if value == "normal" {
			return true
		}
		seen := map[string]bool{}
		for _, part := range strings.Fields(value) {
			if !svgContains([]string{"fill", "stroke", "markers"}, part) || seen[part] {
				return false
			}
			seen[part] = true
		}
		return len(seen) > 0
	case "stroke-dasharray":
		if value == "none" {
			return true
		}
		for _, part := range strings.Fields(strings.ReplaceAll(value, ",", " ")) {
			if !svgNonnegativeLength(part) {
				return false
			}
		}
		for _, part := range strings.Split(value, ",") {
			if strings.TrimSpace(part) == "" {
				return false
			}
		}
		return true
	case "stroke-width":
		return svgNonnegativeLength(value)
	case "stroke-dashoffset":
		return svgStyleLength.MatchString(value)
	case "baseline-shift":
		return oneOf("baseline sub super") || svgStyleLength.MatchString(value)
	case "opacity", "fill-opacity", "stroke-opacity", "stop-opacity", "stroke-miterlimit":
		if !svgStyleNumber.MatchString(value) {
			return false
		}
		number, err := strconv.ParseFloat(value, 64)
		if name == "stroke-miterlimit" {
			return err == nil && number >= 1
		}
		return err == nil && number >= 0 && number <= 1
	}
	return false
}

func svgNonnegativeLength(value string) bool {
	match := svgStyleLength.FindStringSubmatch(value)
	if match == nil {
		return false
	}
	number, err := strconv.ParseFloat(match[1], 64)
	return err == nil && number >= 0
}
