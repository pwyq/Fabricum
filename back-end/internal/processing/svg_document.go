package processing

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

const svgNamespace = "http://www.w3.org/2000/svg"
const xmlNamespace = "http://www.w3.org/XML/1998/namespace"
const xlinkNamespace = "http://www.w3.org/1999/xlink"
const maxSVGBytes = 16 << 20

type svgAttribute struct {
	xml.Attr
	raw string
}

// Raw attribute values and character data are retained, including entity
// spelling, CDATA, quotes, and text spacing. Only markup spacing is normalized.
type svgNode struct {
	name     xml.Name
	qname    string
	attrs    []svgAttribute
	children []*svgNode
	raw      string
}

func svgNamespaceScope(parent map[string]string, n *svgNode) (map[string]string, error) {
	scope := make(map[string]string, len(parent)+2)
	for prefix, uri := range parent {
		scope[prefix] = uri
	}
	for _, attr := range n.attrs {
		prefix := ""
		if attr.Name.Space == "xmlns" {
			prefix = attr.Name.Local
		} else if attr.Name != (xml.Name{Local: "xmlns"}) {
			continue
		}
		if prefix == "xmlns" || prefix == "xml" && attr.Value != xmlNamespace || prefix != "xml" && attr.Value == xmlNamespace || attr.Value == "http://www.w3.org/2000/xmlns/" || prefix != "" && attr.Value == "" {
			return nil, fmt.Errorf("invalid reserved or empty SVG namespace binding")
		}
		scope[prefix] = attr.Value
		if len(scope) > 256 {
			return nil, fmt.Errorf("SVG exceeds in-scope namespace limit")
		}
	}
	names := []string{n.qname}
	for _, attr := range n.attrs {
		names = append(names, strings.SplitN(attr.raw, "=", 2)[0])
	}
	for _, name := range names {
		parts := strings.Split(name, ":")
		if len(parts) > 2 || len(parts) == 2 && parts[0] != "xmlns" && scope[parts[0]] == "" {
			return nil, fmt.Errorf("unbound or invalid SVG namespace prefix in %s", name)
		}
	}
	return scope, nil
}

func (n *svgNode) attribute(name string) string {
	for _, attr := range n.attrs {
		if attr.Name.Space == "" && attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func parseSVG(ctx context.Context, data []byte) (*svgNode, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var root *svgNode
	var stack []*svgNode
	scopes := []map[string]string{{"xml": xmlNamespace}}
	count := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		start := decoder.InputOffset()
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid SVG XML: %w", err)
		}
		count++
		if count > 200000 || len(stack) > 128 {
			return nil, fmt.Errorf("SVG exceeds XML token or nesting limit")
		}
		raw := string(data[start:decoder.InputOffset()])
		switch token := token.(type) {
		case xml.StartElement:
			if len(token.Attr) > 256 {
				return nil, fmt.Errorf("SVG exceeds per-element attribute limit")
			}
			name, attrs, err := svgMarkup(raw, token.Attr)
			if err != nil {
				return nil, err
			}
			node := &svgNode{name: token.Name, qname: name, attrs: attrs}
			scope, err := svgNamespaceScope(scopes[len(scopes)-1], node)
			if err != nil {
				return nil, err
			}
			scopes = append(scopes, scope)
			if len(stack) == 0 {
				if root != nil || token.Name != (xml.Name{Space: svgNamespace, Local: "svg"}) {
					return nil, fmt.Errorf("expected one SVG root with the SVG namespace")
				}
				root = node
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, node)
			}
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("unexpected SVG closing element")
			}
			node := stack[len(stack)-1]
			if raw != "" && strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "</"), ">")) != node.qname {
				return nil, fmt.Errorf("mismatched qualified SVG closing element")
			}
			stack = stack[:len(stack)-1]
			scopes = scopes[:len(scopes)-1]
		case xml.CharData:
			if len(stack) == 0 {
				if strings.TrimSpace(strings.TrimPrefix(string(token), "\ufeff")) != "" {
					return nil, fmt.Errorf("text outside SVG root")
				}
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, &svgNode{raw: raw})
			}
		case xml.Comment:
			// Comments are inert in the supported static SVG profile.
		case xml.ProcInst:
			if token.Target != "xml" || root != nil || start != 0 && !(start == 3 && bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf})) {
				return nil, fmt.Errorf("SVG processing instructions are unsupported (including external stylesheets)")
			}
		case xml.Directive:
			return nil, fmt.Errorf("SVG DTDs and entity declarations are unsupported")
		}
	}
	if root == nil || len(stack) != 0 {
		return nil, fmt.Errorf("missing or incomplete SVG root")
	}
	return root, nil
}

func svgSpace(b byte) bool { return b == ' ' || b == '\n' || b == '\r' || b == '\t' }

// XML has already been validated by encoding/xml. Keep each value's original
// lexeme so serialization cannot change namespace prefixes or text semantics.
func svgMarkup(raw string, attrs []xml.Attr) (string, []svgAttribute, error) {
	i := 1
	for i < len(raw) && !svgSpace(raw[i]) && raw[i] != '/' && raw[i] != '>' {
		i++
	}
	name := raw[1:i]
	result := make([]svgAttribute, 0, len(attrs))
	seen := map[xml.Name]bool{}
	for _, attr := range attrs {
		if seen[attr.Name] {
			return "", nil, fmt.Errorf("duplicate SVG attribute %s", attr.Name.Local)
		}
		seen[attr.Name] = true
		for i < len(raw) && svgSpace(raw[i]) {
			i++
		}
		start := i
		for i < len(raw) && !svgSpace(raw[i]) && raw[i] != '=' {
			i++
		}
		attrName := raw[start:i]
		for i < len(raw) && (svgSpace(raw[i]) || raw[i] == '=') {
			i++
		}
		if i >= len(raw) || raw[i] != '\'' && raw[i] != '"' {
			return "", nil, fmt.Errorf("invalid SVG attribute syntax")
		}
		start = i
		quote := raw[i]
		i++
		for i < len(raw) && raw[i] != quote {
			i++
		}
		i++
		result = append(result, svgAttribute{Attr: attr, raw: attrName + "=" + raw[start:i]})
	}
	return name, result, nil
}

func writeSVG(output *strings.Builder, n *svgNode) {
	if n.qname == "" {
		output.WriteString(n.raw)
		return
	}
	output.WriteByte('<')
	output.WriteString(n.qname)
	for _, attr := range n.attrs {
		output.WriteByte(' ')
		output.WriteString(attr.raw)
	}
	if len(n.children) == 0 {
		output.WriteString("/>")
		return
	}
	output.WriteByte('>')
	for _, child := range n.children {
		writeSVG(output, child)
	}
	output.WriteString("</" + n.qname + ">")
}
