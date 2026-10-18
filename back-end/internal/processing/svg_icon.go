package processing

import (
	"encoding/xml"
	"strings"
)

// Preflight the whole document before changing style specificity. Stylesheets,
// classes, custom properties, CSS transforms, and complex declarations keep the
// conservative profile. Presentation attributes have identical inheritance in
// the supported standalone, selector-free document.
func svgIconStylesSupported(n *svgNode) bool {
	if n.name.Local == "style" || n.attribute("class") != "" {
		return false
	}
	for _, attr := range n.attrs {
		if attr.Name.Space == "" && attr.Name.Local == "class" {
			return false
		}
		if attr.Name.Space == "" && attr.Name.Local == "style" {
			if _, ok := svgInlineDeclarations(attr.Value); !ok {
				return false
			}
		}
	}
	for _, child := range n.children {
		if !svgIconStylesSupported(child) {
			return false
		}
	}
	return true
}

func svgInlineDeclarations(style string) ([][2]string, bool) {
	var declarations [][2]string
	for _, declaration := range strings.Split(style, ";") {
		if strings.TrimSpace(declaration) == "" {
			continue
		}
		name, value, found := strings.Cut(declaration, ":")
		name, value = strings.ToLower(strings.TrimSpace(name)), strings.TrimSpace(value)
		if !found || value == "" || !svgSimpleStyleValue(name, value) {
			return nil, false
		}
		declarations = append(declarations, [2]string{name, value})
	}
	return declarations, true
}

func setSVGAttribute(n *svgNode, name, value string) {
	// All rewritten attributes are unqualified. Escape XML metacharacters while
	// leaving unrelated raw values (including transforms) byte-for-byte intact.
	var escaped strings.Builder
	_ = xml.EscapeText(&escaped, []byte(value))
	attr := svgAttribute{Attr: xml.Attr{Name: xml.Name{Local: name}, Value: value}, raw: name + `="` + escaped.String() + `"`}
	for i := range n.attrs {
		if n.attrs[i].Name == attr.Name {
			n.attrs[i] = attr
			return
		}
	}
	n.attrs = append(n.attrs, attr)
}

func prepareSVGIcon(root *svgNode, policy *svgPolicy) {
	referenced := svgReferencedIDs(policy)
	var visit func(*svgNode)
	visit = func(n *svgNode) {
		declarations, _ := svgInlineDeclarations(n.attribute("style"))
		attrs := n.attrs[:0]
		for _, attr := range n.attrs {
			if svgEditorNamespace(attr.Name.Space) || attr.Name.Space == "" && (attr.Name.Local == "style" || attr.Name.Local == "id" && !referenced[attr.Value] || n == root && attr.Name.Local == "version") {
				continue
			}
			attrs = append(attrs, attr)
		}
		n.attrs = attrs
		for _, declaration := range declarations {
			setSVGAttribute(n, declaration[0], declaration[1])
		}
		children := make([]*svgNode, 0, len(n.children))
		for _, child := range n.children {
			if (child.name.Local == "metadata" || svgEditorNamespace(child.name.Space)) && !svgHasReferencedID(child, referenced) {
				continue
			}
			visit(child)
			if child.name.Local != "defs" || len(child.attrs) != 0 || !svgWhitespaceOnly(child) {
				children = append(children, child)
			}
		}
		n.children = children
	}
	visit(root)
	pruneSVGNamespaces(root)
}

func svgReferencedIDs(policy *svgPolicy) map[string]bool {
	referenced := map[string]bool{}
	for _, id := range policy.references {
		referenced[id] = true
	}
	return referenced
}

func cleanSVGIconPresentation(root *svgNode, policy *svgPolicy) {
	cleanSVGIconAttributes(root, map[string]string{"fill": "black", "stroke": "none", "fill-rule": "nonzero", "fill-opacity": "1", "stroke-opacity": "1", "stroke-dasharray": "none", "marker-start": "none", "marker-mid": "none", "marker-end": "none"}, svgReferencedIDs(policy))
}

func svgHasReferencedID(n *svgNode, references map[string]bool) bool {
	if references[n.attribute("id")] {
		return true
	}
	for _, child := range n.children {
		if svgHasReferencedID(child, references) {
			return true
		}
	}
	return false
}

func svgWhitespaceOnly(n *svgNode) bool {
	for _, child := range n.children {
		if child.qname != "" || strings.TrimSpace(child.raw) != "" {
			return false
		}
	}
	return true
}

func cleanSVGIconAttributes(n *svgNode, inherited map[string]string, referenced map[string]bool) {
	// A use instance inherits from its use element, not the original parent.
	// Keep explicit presentation attributes throughout every referenced subtree.
	if referenced[n.attribute("id")] {
		return
	}
	values := make(map[string]string, len(inherited))
	for name, value := range inherited {
		values[name] = value
	}
	attrs := n.attrs[:0]
	stroke := n.attribute("stroke")
	if stroke == "" {
		stroke = inherited["stroke"]
	}
	markers := false
	for _, name := range []string{"marker-start", "marker-mid", "marker-end"} {
		value := n.attribute(name)
		if value == "" {
			value = inherited[name]
		}
		markers = markers || value != "none"
	}
	for _, attr := range n.attrs {
		name, value := attr.Name.Local, attr.Value
		remove := false
		if attr.Name.Space == "" {
			if previous, ok := inherited[name]; ok {
				remove = svgPaintKey(previous) == svgPaintKey(value)
				values[name] = value
			}
			remove = remove || name == "opacity" && value == "1"
			if n.name.Local == "path" {
				remove = remove || stroke == "none" && strings.HasPrefix(name, "stroke-") && (name != "stroke-width" || !markers)
				remove = remove || name == "baseline-shift" && value == "baseline" || name == "vector-effect" && value == "none" || name == "display" && value == "inline" || name == "overflow" && value == "visible" || name == "enable-background" && value == "accumulate" || name == "stop-color"
			}
		}
		if !remove {
			attrs = append(attrs, attr)
		}
	}
	n.attrs = attrs
	for _, child := range n.children {
		cleanSVGIconAttributes(child, values, referenced)
	}
}

func pruneSVGNamespaces(n *svgNode) {
	attrs := n.attrs[:0]
	for _, attr := range n.attrs {
		if attr.Name.Space != "xmlns" || svgPrefixUsed(n, attr.Name.Local, true) {
			attrs = append(attrs, attr)
		}
	}
	n.attrs = attrs
	for _, child := range n.children {
		pruneSVGNamespaces(child)
	}
}

func svgPrefixUsed(n *svgNode, prefix string, own bool) bool {
	if !own {
		for _, attr := range n.attrs {
			if attr.Name.Space == "xmlns" && attr.Name.Local == prefix {
				return false // A nested declaration shadows this binding.
			}
		}
	}
	if strings.HasPrefix(n.qname, prefix+":") {
		return true
	}
	for _, attr := range n.attrs {
		if strings.HasPrefix(attr.raw, prefix+":") {
			return true
		}
	}
	for _, child := range n.children {
		if svgPrefixUsed(child, prefix, false) {
			return true
		}
	}
	return false
}
