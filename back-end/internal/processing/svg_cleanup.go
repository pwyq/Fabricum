package processing

import (
	"encoding/xml"
	"strings"
)

func cleanSVG(n *svgNode, preserveSpace bool) {
	if n.qname == "" {
		return
	}
	for _, attr := range n.attrs {
		if attr.Name == (xml.Name{Space: xmlNamespace, Local: "space"}) {
			preserveSpace = attr.Value != "default"
		}
	}
	attrs := n.attrs[:0]
	for _, attr := range n.attrs {
		if !svgEditorNamespace(attr.Name.Space) && !svgDefaultAttribute(n, attr) {
			attrs = append(attrs, attr)
		}
	}
	n.attrs = attrs
	// Unwrapping can add more than one child. A fresh slice avoids overwriting
	// siblings that the loop has not visited yet.
	children := make([]*svgNode, 0, len(n.children))
	for _, child := range n.children {
		if (child.name == (xml.Name{Space: svgNamespace, Local: "metadata"}) || svgEditorNamespace(child.name.Space)) && !svgHasID(child) {
			continue
		}
		if !preserveSpace && svgElementOnly(n.name.Local) && child.qname == "" && strings.TrimSpace(child.raw) == "" {
			continue
		}
		cleanSVG(child, preserveSpace)
		if (n.name.Local == "svg" || n.name.Local == "g") && child.name.Local == "g" && len(child.attrs) == 0 && svgOnlyGraphics(child) {
			children = append(children, child.children...)
		} else {
			children = append(children, child)
		}
	}
	n.children = children
	if n.name.Local == "defs" {
		shareSVGGradients(n)
	}
}

func svgHasID(n *svgNode) bool {
	for _, attr := range n.attrs {
		if attr.Name.Local == "id" {
			return true
		}
	}
	for _, child := range n.children {
		if svgHasID(child) {
			return true
		}
	}
	return false
}

func svgOnlyGraphics(n *svgNode) bool {
	for _, child := range n.children {
		if child.qname != "" && !svgContains(strings.Fields("path rect circle ellipse line polyline polygon g use text"), child.name.Local) {
			return false
		}
		if child.name.Local == "g" && !svgOnlyGraphics(child) {
			return false
		}
	}
	return true
}

func svgElementOnly(name string) bool {
	return svgContains(strings.Fields("svg g defs symbol switch clipPath mask marker pattern filter linearGradient radialGradient"), name)
}

func svgDefaultAttribute(n *svgNode, attr svgAttribute) bool {
	if attr.Name.Space != "" {
		return false
	}
	// A local explicit default may override a template reached through href.
	for _, other := range n.attrs {
		if other.Name.Local == "href" {
			return false
		}
	}
	if n.name.Local == "g" && attr.Name.Local == "transform" {
		return svgContains([]string{"translate(0)", "translate(0 0)", "translate(0,0)", "matrix(1 0 0 1 0 0)", "scale(1)"}, attr.Value)
	}
	defaults := map[string]map[string]string{
		"linearGradient": {"gradientUnits": "objectBoundingBox", "spreadMethod": "pad"},
		"radialGradient": {"gradientUnits": "objectBoundingBox", "spreadMethod": "pad"},
		"clipPath":       {"clipPathUnits": "userSpaceOnUse"},
		"mask":           {"maskUnits": "objectBoundingBox", "maskContentUnits": "userSpaceOnUse"},
		"pattern":        {"patternUnits": "objectBoundingBox", "patternContentUnits": "userSpaceOnUse"},
		"filter":         {"filterUnits": "objectBoundingBox", "primitiveUnits": "userSpaceOnUse"},
	}
	value, ok := defaults[n.name.Local][attr.Name.Local]
	return ok && value == attr.Value
}

// SVG2 gradient templates share identical stop lists without renaming IDs.
// Masks, filters, geometry, and definitions with descendant IDs stay intact.
func shareSVGGradients(defs *svgNode) {
	seen := map[string]string{}
	for _, n := range defs.children {
		id := n.attribute("id")
		if id == "" || n.name.Local != "linearGradient" && n.name.Local != "radialGradient" {
			continue
		}
		eligible := true
		for _, child := range n.children {
			if child.name.Local != "stop" || len(child.children) != 0 || svgHasID(child) {
				eligible = false
			}
		}
		for _, attr := range n.attrs {
			if attr.Name.Local == "href" {
				eligible = false
			}
		}
		if !eligible {
			continue
		}
		copy := *n
		copy.attrs = nil
		for _, attr := range n.attrs {
			if attr.Name.Local != "id" {
				copy.attrs = append(copy.attrs, attr)
			}
		}
		var key strings.Builder
		writeSVG(&key, &copy)
		if previous, ok := seen[key.String()]; ok {
			copy = *n
			copy.children = nil
			copy.attrs = append(append([]svgAttribute{}, n.attrs...), svgAttribute{Attr: xml.Attr{Name: xml.Name{Local: "href"}, Value: "#" + previous}, raw: `href="#` + previous + `"`})
			var before, after strings.Builder
			writeSVG(&before, n)
			writeSVG(&after, &copy)
			if after.Len() < before.Len() {
				*n = copy
			}
		} else {
			seen[key.String()] = id
		}
	}
}
