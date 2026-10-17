package processing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSVGOptimizationFixtures(t *testing.T) {
	for _, name := range []string{"icon", "references", "text-style", "prefixed"} {
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", "svg", name+".svg"))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "export with spaces.svg")
			writePNGTestFile(t, path, input)
			result, err := OptimizeSVG(nil, SVGOptimizationRequest{Path: path})
			if err != nil || !result.Changed || result.OutputBytes >= len(input) || result.SavingsBytes != len(input)-result.OutputBytes || result.SavingsPercent <= 0 || result.Format != "svg" || result.Optimizer != "fabricum-svg" || result.OptimizerVersion != SVGOptimizerVersion {
				t.Fatalf("result: %+v, %v", result, err)
			}
			output, _ := os.ReadFile(path)
			if bytes.Contains(output, []byte("<!--")) || !bytes.Contains(output, []byte("viewBox=")) {
				t.Fatal("comment cleanup or viewBox preservation failed")
			}
			if name == "icon" && (bytes.Contains(output, []byte("<g")) || bytes.Contains(output, []byte("metadata")) || !bytes.Contains(output, []byte("20.123456789"))) {
				t.Fatal("structure cleanup or numeric precision changed")
			}
			if name == "references" && (!bytes.Contains(output, []byte(`id="duplicate" href="#paint"`)) || !bytes.Contains(output, []byte(`xlink:href="#triangle"`))) {
				t.Fatalf("gradient sharing or reference preservation failed: %s", output)
			}
			if name == "text-style" && (len(result.Diagnostics) == 0 || !bytes.Contains(output, []byte("<g>\n")) || !bytes.Contains(output, []byte(` A <tspan class="accent"> B </tspan> C &amp; D `))) {
				t.Fatal("CSS structure or text spacing changed")
			}
			if name == "prefixed" && !bytes.Contains(output, []byte(`A <s:tspan>B</s:tspan> C&#32;D`)) {
				t.Fatal("qualified names or significant text changed")
			}
			info, _ := os.Stat(path)
			if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
				t.Fatal("permissions changed")
			}
			old := time.Unix(1234567890, 0)
			if err := os.Chtimes(path, old, old); err != nil {
				t.Fatal(err)
			}
			again, err := OptimizeSVG(nil, SVGOptimizationRequest{Path: path})
			if err != nil || again.Changed || again.SavingsBytes != 0 || again.OutputBytes != len(output) {
				t.Fatalf("not idempotent: %+v, %v", again, err)
			}
			repeated, _ := os.ReadFile(path)
			info, _ = os.Stat(path)
			if !bytes.Equal(output, repeated) || !info.ModTime().Equal(old) {
				t.Fatal("no-op rewrote file")
			}
		})
	}
}

func TestSVGUnsafeAndMalformedInputsRetainOriginal(t *testing.T) {
	for name, body := range map[string]string{
		"script":                `<script>alert(1)</script>`,
		"foreign-object":        `<foreignObject/>`,
		"animation":             `<animate attributeName="href" values="https://example.com"/>`,
		"event":                 `<rect onload="alert(1)"/>`,
		"external":              `<use href="https://example.com/icon.svg#id"/>`,
		"file":                  `<use href="file:///etc/passwd"/>`,
		"relative":              `<use href="other.svg#id"/>`,
		"data":                  `<use href="data:image/svg+xml;base64,AAAA"/>`,
		"entity-url":            `<use href="&#104;ttps://example.com"/>`,
		"paint-url":             `<rect fill="url(https://example.com)"/>`,
		"css-import":            `<style>@import 'https://example.com';</style>`,
		"css-escape":            `<style>rect{fill:u\72l(https://example.com)}</style>`,
		"css-comment":           `<style>rect{fill:u/**/rl(https://example.com)}</style>`,
		"css-image":             `<style>rect{fill:image-set('https://example.com' 1x)}</style>`,
		"css-entity":            `<style>&#64;import 'https://example.com';</style>`,
		"css-cdata":             `<style><![CDATA[@import 'https://example.com';]]></style>`,
		"css-xml-comment":       `<style>rect{fill:u<!--split-->rl(https://example.com)}</style>`,
		"base":                  `<g xml:base="https://example.com"/>`,
		"missing-reference":     `<use href="#missing"/>`,
		"missing-css-reference": `<rect style="fill:url('#missing')"/>`,
		"duplicate-id":          `<g id="a"/><g id="a"/>`,
		"duplicate-attribute":   `<rect width="1" width="2"/>`,
		"unknown-element":       `<unknown/>`,
		"unknown-namespace":     `<x:rect xmlns:x="urn:unknown"/>`,
		"active-metadata":       `<metadata><iframe xmlns="http://www.w3.org/1999/xhtml" srcdoc="&lt;script&gt;alert(1)&lt;/script&gt;"/></metadata>`,
		"metadata-object":       `<metadata><object xmlns="http://www.w3.org/1999/xhtml" data="https://example.com"/></metadata>`,
		"metadata-unbound":      `<metadata><x:item/></metadata>`,
		"unbound-prefix":        `<x:rect/>`,
		"broken-xml":            `<g>`,
		"entity":                `<text>&unknown;</text>`,
	} {
		t.Run(name, func(t *testing.T) {
			assertSVGFailureUnchanged(t, `<svg xmlns="`+svgNamespace+`">`+body+`</svg>`)
		})
	}
	for _, input := range []string{
		`<!DOCTYPE svg SYSTEM "https://example.com/evil.dtd"><svg xmlns="` + svgNamespace + `"/>`,
		`<!DOCTYPE svg [<!ENTITY x SYSTEM "file:///etc/passwd">]><svg xmlns="` + svgNamespace + `"/>`,
		`<?xml-stylesheet href="https://example.com"?><svg xmlns="` + svgNamespace + `"/>`,
		`<svg/>`, `<svg xmlns="` + svgNamespace + `"/><svg xmlns="` + svgNamespace + `"/>`,
		`<svg xmlns="` + svgNamespace + `"/>trailing`,
		`<svg xmlns="` + svgNamespace + `">` + strings.Repeat("<g>", 130) + strings.Repeat("</g>", 130) + `</svg>`,
	} {
		assertSVGFailureUnchanged(t, input)
	}
}

func assertSVGFailureUnchanged(t *testing.T, input string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "invalid.svg")
	writePNGTestFile(t, path, []byte(input))
	old := time.Unix(1234567890, 0)
	_ = os.Chtimes(path, old, old)
	result, err := OptimizeSVG(nil, SVGOptimizationRequest{Path: path})
	if err == nil || result.Changed || result.SavingsBytes != 0 {
		t.Fatalf("accepted unsafe/malformed input: %+v, %v", result, err)
	}
	after, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if string(after) != input || !info.ModTime().Equal(old) {
		t.Fatal("error changed original")
	}
}

func TestSVGPreservesSensitiveStructures(t *testing.T) {
	for _, body := range []string{
		`<style>g > rect {fill: red}</style><g transform="translate(0)"><rect/></g>`,
		`<g class="external"><rect/></g>`,
		`<metadata id="credit">author</metadata><title id="title">Keep me</title><g aria-labelledby="credit title"/>`,
		`<switch><g><rect/></g><circle/></switch>`,
		`<g><title>Semantic group</title><rect/></g>`,
		`<defs><linearGradient id="a" gradientUnits="userSpaceOnUse"/><linearGradient id="b" href="#a" gradientUnits="objectBoundingBox"/></defs>`,
		`<text xml:space="preserve"> A <tspan> B </tspan> C </text>`,
	} {
		input := `<svg xmlns="` + svgNamespace + `">` + body + `</svg>`
		output, _, err := optimizeSVGDocument(context.Background(), []byte(input))
		if err != nil || string(output) != input {
			t.Fatalf("sensitive structure changed: %s -> %s (%v)", input, output, err)
		}
	}
}

func TestSVGGroupUnwrappingPreservesFollowingSiblings(t *testing.T) {
	input := `<svg xmlns="` + svgNamespace + `"><g><rect id="first"/><path id="second"/></g><circle id="third"/></svg>`
	expected := `<svg xmlns="` + svgNamespace + `"><rect id="first"/><path id="second"/><circle id="third"/></svg>`
	output, _, err := optimizeSVGDocument(context.Background(), []byte(input))
	if err != nil || string(output) != expected {
		t.Fatalf("lost or reordered sibling: %s, %v", output, err)
	}
}

func TestSVGPathAndResourceLimits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := OptimizeSVG(ctx, SVGOptimizationRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, path := range []string{"", t.TempDir(), filepath.Join(t.TempDir(), "missing.svg")} {
		if _, err := OptimizeSVG(nil, SVGOptimizationRequest{Path: path}); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
	path := filepath.Join(t.TempDir(), "huge.svg")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	err = file.Truncate(maxSVGBytes + 1)
	_ = file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OptimizeSVG(nil, SVGOptimizationRequest{Path: path}); err == nil {
		t.Fatal("accepted oversized input")
	}
	link := filepath.Join(t.TempDir(), "link.svg")
	if err := os.Symlink(path, link); err == nil {
		if _, err := OptimizeSVG(nil, SVGOptimizationRequest{Path: link}); err == nil {
			t.Fatal("accepted symbolic link")
		}
	}
}

func TestSVGBoundsNamespaceScope(t *testing.T) {
	var input strings.Builder
	input.WriteString(`<svg xmlns="` + svgNamespace + `">`)
	for level := 0; level < 3; level++ {
		input.WriteString("<g")
		for prefix := 0; prefix < 100; prefix++ {
			fmt.Fprintf(&input, ` xmlns:n%d_%d="urn:metadata"`, level, prefix)
		}
		input.WriteString(">")
	}
	input.WriteString("</g></g></g></svg>")
	assertSVGFailureUnchanged(t, input.String())
}
