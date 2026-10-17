package processing

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

var svgElements = strings.Fields(`svg g defs symbol use a switch title desc metadata
 path rect circle ellipse line polyline polygon text tspan textPath style
 linearGradient radialGradient stop pattern clipPath mask marker filter
 feBlend feColorMatrix feComponentTransfer feComposite feConvolveMatrix
 feDiffuseLighting feDisplacementMap feDistantLight feDropShadow feFlood
 feFuncA feFuncB feFuncG feFuncR feGaussianBlur feImage feMerge feMergeNode
 feMorphology feOffset fePointLight feSpecularLighting feSpotLight feTile feTurbulence`)

var svgID = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.:-]*$`)
var svgURL = regexp.MustCompile(`(?i)url\s*\(\s*([^)]*)\)`)
var svgFunction = regexp.MustCompile(`([A-Za-z_-][A-Za-z0-9_-]*)\s*\(`)

type svgPolicy struct {
	styled      bool
	diagnostics []string
	ids         map[string]bool
	references  []string
}

func svgEditorNamespace(namespace string) bool {
	return svgContains([]string{
		"http://www.inkscape.org/namespaces/inkscape",
		"http://sodipodi.sourceforge.net/DTD/sodipodi-0.dtd",
		"http://ns.adobe.com/AdobeIllustrator/10.0/",
		"http://ns.adobe.com/AdobeSVGViewerExtensions/3.0/",
	}, namespace)
}

func validateSVG(root *svgNode) (*svgPolicy, error) {
	policy := &svgPolicy{ids: map[string]bool{}}
	if err := policy.visit(root, false); err != nil {
		return nil, err
	}
	for _, id := range policy.references {
		if !policy.ids[id] {
			return nil, fmt.Errorf("SVG reference #%s has no matching ID", id)
		}
	}
	if policy.styled {
		policy.diagnostics = append(policy.diagnostics, "Styles/classes preserved verbatim; structure, metadata, default attributes, and character whitespace retained for selector and text behavior.")
	}
	return policy, nil
}

func (p *svgPolicy) visit(n *svgNode, metadata bool) error {
	if n.qname == "" {
		return nil
	}
	local := strings.ToLower(n.name.Local)
	if local == "script" || local == "foreignobject" || strings.HasPrefix(local, "animate") || local == "set" || local == "discard" {
		return fmt.Errorf("unsafe or unsupported SVG element %s", n.qname)
	}
	if !metadata && !svgEditorNamespace(n.name.Space) && (n.name.Space != svgNamespace || !svgContains(svgElements, n.name.Local)) {
		return fmt.Errorf("unsupported SVG element or namespace %s", n.qname)
	}
	metadata = metadata || n.name.Local == "metadata" || svgEditorNamespace(n.name.Space)
	for _, attr := range n.attrs {
		name, value := strings.ToLower(attr.Name.Local), attr.Value
		if attr.Name.Space == "xmlns" || attr.Name == (xml.Name{Local: "xmlns"}) {
			if strings.HasPrefix(attr.raw, "xmlns:xml=") && value != xmlNamespace || strings.HasPrefix(attr.raw, "xmlns:xmlns=") || value == "http://www.w3.org/2000/xmlns/" {
				return fmt.Errorf("invalid reserved SVG namespace binding")
			}
			continue
		}
		if strings.HasPrefix(name, "on") || name == "base" || name == "src" {
			return fmt.Errorf("unsafe SVG attribute %s", attr.Name.Local)
		}
		if !metadata && attr.Name.Space != "" && attr.Name.Space != xmlNamespace && attr.Name.Space != xlinkNamespace && !svgEditorNamespace(attr.Name.Space) {
			return fmt.Errorf("unsupported SVG attribute namespace %s", attr.Name.Space)
		}
		if name == "href" {
			if err := p.fragment(value); err != nil {
				return err
			}
		}
		if name == "id" {
			if !svgID.MatchString(value) || p.ids[value] {
				return fmt.Errorf("unsupported or duplicate SVG ID %q", value)
			}
			p.ids[value] = true
		}
		if name == "aria-labelledby" || name == "aria-describedby" {
			p.references = append(p.references, strings.Fields(value)...)
		}
		if name == "style" || name == "class" {
			p.styled = true
		}
		if err := p.css(value); err != nil {
			return fmt.Errorf("SVG attribute %s: %w", attr.Name.Local, err)
		}
	}
	if n.name.Local == "style" {
		p.styled = true
		if kind := n.attribute("type"); kind != "" && kind != "text/css" {
			return fmt.Errorf("unsupported SVG stylesheet type %q", kind)
		}
		var text strings.Builder
		for _, child := range n.children {
			if child.qname != "" {
				return fmt.Errorf("SVG style must contain only CSS text")
			}
			// Decode entities and CDATA before checking CSS, just as a renderer does.
			decoder := xml.NewDecoder(strings.NewReader("<x>" + child.raw + "</x>"))
			for token, err := decoder.Token(); err == nil; token, err = decoder.Token() {
				if data, ok := token.(xml.CharData); ok {
					text.Write(data)
				}
			}
		}
		if err := p.css(text.String()); err != nil {
			return fmt.Errorf("SVG style: %w", err)
		}
	}
	for _, child := range n.children {
		if err := p.visit(child, metadata); err != nil {
			return err
		}
	}
	return nil
}

func (p *svgPolicy) fragment(value string) error {
	if !strings.HasPrefix(value, "#") || !svgID.MatchString(strings.TrimPrefix(value, "#")) {
		return fmt.Errorf("only local SVG fragment references are supported; external, file, and data resources are rejected")
	}
	p.references = append(p.references, value[1:])
	return nil
}

func (p *svgPolicy) css(value string) error {
	// Fail closed on escapes, comments and at-rules: they can conceal URLs,
	// imports, scripts, font downloads, or resource-producing CSS functions.
	if strings.ContainsAny(value, "\\@") || strings.Contains(value, "/*") {
		return fmt.Errorf("CSS escapes, comments, and at-rules are unsupported in offline SVG")
	}
	for _, match := range svgURL.FindAllStringSubmatch(value, -1) {
		if err := p.fragment(strings.Trim(strings.TrimSpace(match[1]), "\"'")); err != nil {
			return err
		}
	}
	for _, match := range svgFunction.FindAllStringSubmatch(value, -1) {
		switch strings.ToLower(match[1]) {
		case "url", "rgb", "rgba", "hsl", "hsla", "hwb", "lab", "lch", "oklab", "oklch", "color", "color-mix", "var", "calc", "min", "max", "clamp", "matrix", "translate", "translatex", "translatey", "scale", "scalex", "scaley", "rotate", "skewx", "skewy":
		default:
			return fmt.Errorf("unsupported SVG/CSS function %s", match[1])
		}
	}
	return nil
}

func svgContains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
