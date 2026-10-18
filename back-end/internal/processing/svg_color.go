package processing

import "strings"

func svgPaintKey(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "#000", "#000000", "black", "rgb(0,0,0)", "rgb(0, 0, 0)":
		return "black"
	case "#fff", "#ffffff", "white":
		return "white"
	}
	return value
}

// Only rewrite simple painted geometry. Definitions, text and inherited CSS
// colors can give identical paint strings different meanings. Multiple paints
// (including white overlays) are retained in full.
func svgCurrentColor(root *svgNode) bool {
	paints := map[string]bool{}
	var visit func(*svgNode, string, string) bool
	visit = func(n *svgNode, fill, stroke string) bool {
		if n.qname == "" {
			return true
		}
		if !svgContains(strings.Fields("svg g path rect circle ellipse line polyline polygon title desc"), n.name.Local) {
			return false
		}
		if value := n.attribute("fill"); value != "" {
			fill = value
		}
		if value := n.attribute("stroke"); value != "" {
			stroke = value
		}
		for _, paint := range []string{fill, stroke} {
			if paint != "none" && !svgOpaquePaint(paint) {
				return false
			}
		}
		if svgContains(strings.Fields("path rect circle ellipse line polyline polygon"), n.name.Local) {
			if fill != "none" && n.name.Local != "line" {
				paints[svgPaintKey(fill)] = true
			}
			if stroke != "none" {
				paints[svgPaintKey(stroke)] = true
			}
		}
		for _, child := range n.children {
			if !visit(child, fill, stroke) {
				return false
			}
		}
		return true
	}
	if !visit(root, "black", "none") || len(paints) != 1 {
		return false
	}
	var rewrite func(*svgNode)
	rewrite = func(n *svgNode) {
		for _, name := range []string{"fill", "stroke"} {
			value := n.attribute(name)
			if value != "" && value != "none" {
				setSVGAttribute(n, name, "currentColor")
			}
		}
		for _, child := range n.children {
			rewrite(child)
		}
	}
	rewrite(root)
	if root.attribute("fill") == "" {
		setSVGAttribute(root, "fill", "currentColor")
	}
	return true
}

func svgOpaquePaint(value string) bool {
	if !svgSimpleColor(value) || strings.EqualFold(value, "transparent") {
		return false
	}
	if strings.HasPrefix(value, "#") {
		if len(value) == 5 {
			return strings.EqualFold(value[4:], "f")
		}
		if len(value) == 9 {
			return strings.EqualFold(value[7:], "ff")
		}
	}
	return true
}
