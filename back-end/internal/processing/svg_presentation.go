package processing

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Explicit precision also rounds literal stroke widths. Viewports use whole
// user units independently of the chosen coordinate precision. Selective
// rounding remains confined to path data.
func roundSVGPresentation(n *svgNode, options SVGOptimizationOptions) error {
	if options.Precision == nil {
		return nil
	}
	if n.name.Space == svgNamespace {
		if value := n.attribute("stroke-width"); value != "" {
			if match := svgStyleLength.FindStringSubmatch(strings.TrimSpace(value)); match != nil {
				number, err := strconv.ParseFloat(match[1], 64)
				if err == nil && number >= 0 {
					// Keep units, and do not apply the path-only selective rule.
					rounded, _ := roundSVGNumber(match[1], number, SVGOptimizationOptions{Precision: options.Precision})
					setSVGAttribute(n, "stroke-width", rounded+strings.TrimSpace(value)[len(match[1]):])
				}
			}
		}
		if value := n.attribute("viewBox"); value != "" {
			rounded, err := roundSVGViewBox(value)
			if err != nil {
				return err
			}
			setSVGAttribute(n, "viewBox", rounded)
		}
	}
	for _, child := range n.children {
		if err := roundSVGPresentation(child, options); err != nil {
			return err
		}
	}
	return nil
}

func roundSVGViewBox(value string) (string, error) {
	for _, part := range strings.Split(value, ",") {
		if strings.TrimSpace(part) == "" {
			return "", fmt.Errorf("SVG viewBox must contain four finite numbers")
		}
	}
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r' })
	if len(parts) != 4 {
		return "", fmt.Errorf("SVG viewBox must contain four finite numbers")
	}
	for i, part := range parts {
		number, err := strconv.ParseFloat(part, 64)
		if err != nil || !svgStyleNumber.MatchString(part) || math.IsNaN(number) || math.IsInf(number, 0) {
			return "", fmt.Errorf("SVG viewBox must contain four finite numbers")
		}
		rounded := math.Round(number)
		if i >= 2 && (number <= 0 || rounded <= 0) {
			return "", fmt.Errorf("SVG viewBox width and height must remain positive after integer rounding")
		}
		parts[i] = compactSVGNumber(strconv.FormatFloat(rounded, 'f', 0, 64))
	}
	return strings.Join(parts, " "), nil
}
