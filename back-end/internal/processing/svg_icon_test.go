package processing

import (
	"bytes"
	"context"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func svgTestOptions() SVGOptimizationOptions {
	precision := 2
	return SVGOptimizationOptions{Icon: true, Precision: &precision, SelectiveRounding: true, CurrentColor: true}
}

func TestSVGSelectiveRounding(t *testing.T) {
	round := func(value float64, options SVGOptimizationOptions) float64 {
		_, rounded := roundSVGNumber(strconv.FormatFloat(value, 'f', -1, 64), value, options)
		return rounded
	}
	for input, want := range map[float64]float64{
		.999: 1, .99: 1, .89: .9, .21: .2, .01: 0, .11: .1,
		.991: 1, .119: .12, .123: .123, .12: .12, .25: .25,
		.001: 0, .009: 0, 1.01: 1, 100.1231: 100.1231,
	} {
		for _, sign := range []float64{1, -1} {
			got := round(sign*input, SVGOptimizationOptions{SelectiveRounding: true})
			if got != sign*want {
				t.Fatalf("round(%v) = %v, want %v", sign*input, got, sign*want)
			}
			if round(got, SVGOptimizationOptions{SelectiveRounding: true}) != got {
				t.Fatal("unstable rounding")
			}
		}
	}
	precision := 2
	if got := round(.125, SVGOptimizationOptions{Precision: &precision}); got != .13 {
		t.Fatalf("precision rounding: %v", got)
	}
	if got := round(.8897, svgTestOptions()); got != .9 {
		t.Fatalf("rounding order: %v", got)
	}
}

func TestSVGIconReferenceExport(t *testing.T) {
	input := svgTestRead(t, "inkscape-icon.svg")
	output, diagnostics, reports, err := optimizeSVGWithOptions(context.Background(), input, svgTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 || len(reports) != 7 || len(output) > 2800 {
		t.Fatalf("unexpected cleanup: %d bytes, %+v, %+v", len(output), diagnostics, reports)
	}
	for _, removed := range []string{"inkscape", "sodipodi", "<defs", " id=", " style=", "\n", "<!--"} {
		if bytes.Contains(output, []byte(removed)) {
			t.Fatalf("retained %q: %s", removed, output)
		}
	}
	for _, kept := range []string{`viewBox="0 0 254 254"`, `width="960"`, `height="960"`, `stroke-width="11.53"`, `fill="none"`, `stroke="currentColor"`, `fill="currentColor"`, `transform="matrix(1.1622963,0,0,1.1622963,-30.415523,-19.620168)"`} {
		if !bytes.Contains(output, []byte(kept)) {
			t.Fatalf("lost %q", kept)
		}
	}
	if bytes.Count(output, []byte("<g ")) != 1 || bytes.Count(output, []byte("<path ")) != 7 || bytes.Count(output, []byte("c")) == 0 {
		t.Fatal("lost shared group/paths")
	}
	before, after := svgTestPaths(t, input), svgTestPaths(t, output)
	curves := 0
	maxBound := 0.0
	for i := range before {
		a, b := svgTestCubicPoints(t, before[i]), svgTestCubicPoints(t, after[i])
		if len(a) != len(b) {
			t.Fatal("curve structure changed")
		}
		curves += len(a)
		bound := reports[i].MaxDeviation
		maxBound = math.Max(maxBound, bound)
		for j := range a {
			for sample := 0; sample <= 100; sample++ {
				u := float64(sample) / 100
				p, q := svgTestBezier(a[j], u), svgTestBezier(b[j], u)
				if math.Hypot(p.x-q.x, p.y-q.y) > bound+1e-10 {
					t.Fatal("curve sample exceeds reported convex-hull bound")
				}
			}
		}
	}
	if curves != 78 {
		t.Fatalf("lost cubic segments: %d", curves)
	}
	for _, size := range []float64{24, 960} {
		// The original viewport's aspect-ratio centering also shifts by half
		// the width adjustment. Both before/after use the 254-high meet scale.
		viewportShift := math.Abs(254-253.99999) * size / 254 / 2
		pixels := maxBound*1.1622963*size/254 + viewportShift
		t.Logf("centerline bound at %.0fpx: %.6fpx", size, pixels)
		if pixels > .5*size/960 {
			t.Fatalf("excessive geometry drift: %.6fpx", pixels)
		}
	}
	again, _, _, err := optimizeSVGWithOptions(context.Background(), output, svgTestOptions())
	if err != nil || !bytes.Equal(output, again) {
		t.Fatalf("unstable icon output: %v\n%s", err, again)
	}
}

func TestSVGSelectiveReferenceComparison(t *testing.T) {
	input, expected := svgTestRead(t, "inkscape-icon.optimized.svg"), svgTestRead(t, "inkscape-icon.rounded.svg")
	output, _, _, err := optimizeSVGWithOptions(context.Background(), input, SVGOptimizationOptions{SelectiveRounding: true})
	if err != nil {
		t.Fatal(err)
	}
	got, want := svgTestPaths(t, output), svgTestPaths(t, expected)
	if len(got) != len(want) {
		t.Fatal("path count changed")
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("path %d disagrees with supplied comparison:\n%s\n%s", i, got[i], want[i])
		}
	}
	// Only d is rewritten; all remaining attributes must match the input.
	a, _ := parseSVG(context.Background(), input)
	b, _ := parseSVG(context.Background(), output)
	var strip func(*svgNode)
	strip = func(n *svgNode) {
		if n.name.Local == "path" {
			setSVGAttribute(n, "d", "")
		}
		for _, child := range n.children {
			strip(child)
		}
	}
	strip(a)
	strip(b)
	var x, y strings.Builder
	writeSVG(&x, a)
	writeSVG(&y, b)
	if x.String() != y.String() {
		t.Fatal("selective rounding changed non-path data")
	}
}

func TestSVGIconReferencesColorsAndStyles(t *testing.T) {
	for name, test := range map[string]struct {
		body          string
		kept, removed []string
		diagnostics   bool
	}{
		"references":        {`<title id="label">Icon</title><defs><linearGradient id="paint"><stop stop-color="red"/></linearGradient><path id="shape" d="M0 0L1 1"/></defs><g aria-labelledby="label"><use href="#shape" fill="url(#paint)"/></g>`, []string{`id="label"`, `id="paint"`, `id="shape"`, `href="#shape"`, `url(#paint)`}, nil, false},
		"white-overlay":     {`<path d="M0 0L1 1"/><path d="M2 2L3 3" fill="white" opacity=".5" fill-rule="evenodd"/>`, []string{`fill="white"`, `opacity=".5"`, `fill-rule="evenodd"`}, []string{"currentColor"}, true},
		"alpha-paint":       {`<path d="M0 0L1 1" fill="#ff000080"/>`, []string{`fill="#ff000080"`}, []string{"currentColor"}, true},
		"transparent-paint": {`<path d="M0 0L1 1" fill="transparent"/>`, []string{`fill="transparent"`}, []string{"currentColor"}, true},
		"inline-cascade":    {`<g style="fill:red;opacity:.5"><path fill="green" style="fill:blue;fill:red;fill-opacity:.7" d="M0 0L1 1"/></g>`, []string{`opacity=".5"`, `fill-opacity=".7"`, `currentColor`}, []string{"style=", "green", "blue"}, false},
		"inheritance":       {`<g fill="red" stroke="blue" fill-rule="evenodd"><path fill="black" stroke="none" fill-rule="nonzero" d="M0 0L1 1"/></g>`, []string{`fill="currentColor"`, `stroke="none"`, `fill-rule="nonzero"`}, nil, false},
	} {
		t.Run(name, func(t *testing.T) {
			options := SVGOptimizationOptions{Icon: true, CurrentColor: true}
			output, diagnostics, _, err := optimizeSVGWithOptions(context.Background(), []byte(`<svg xmlns="`+svgNamespace+`">`+test.body+`</svg>`), options)
			if err != nil {
				t.Fatal(err)
			}
			for _, value := range test.kept {
				if !bytes.Contains(output, []byte(value)) {
					t.Fatalf("lost %q: %s", value, output)
				}
			}
			for _, value := range test.removed {
				if bytes.Contains(output, []byte(value)) {
					t.Fatalf("retained %q: %s", value, output)
				}
			}
			if test.diagnostics && len(diagnostics) == 0 {
				t.Fatal("missing preservation diagnostic")
			}
			again, _, _, err := optimizeSVGWithOptions(context.Background(), output, options)
			if err != nil || !bytes.Equal(output, again) {
				t.Fatalf("unstable icon: %s -> %s (%v)", output, again, err)
			}
		})
	}
	for _, body := range []string{`<style>path[d="M 0,0"]{fill:red}</style><g><path d="M 0,0"/></g>`, `<g class="x"><path d="M 0,0"/></g>`, `<g style="--paint:red"><path d="M 0,0"/></g>`, `<g style="fill:red!important"><path d="M 0,0"/></g>`, `<path fill="red" style="fill:invalid" d="M 0,0"/>`, `<path style="fill:red;fill:invalid" d="M 0,0"/>`} {
		input := []byte(`<svg xmlns="` + svgNamespace + `">` + body + `</svg>`)
		output, diagnostics, _, err := optimizeSVGWithOptions(context.Background(), input, SVGOptimizationOptions{Icon: true})
		if err != nil || len(diagnostics) == 0 || !bytes.Equal(input, output) {
			t.Fatalf("unsafe style rewrite: %s, %v", output, err)
		}
		if _, _, _, err := optimizeSVGWithOptions(context.Background(), input, svgTestOptions()); err == nil {
			t.Fatal("accepted rounding under selectors/complex styles")
		}
	}
}

func TestSVGSeparateOutputSafety(t *testing.T) {
	directory := t.TempDir()
	source, destination := filepath.Join(directory, "authoring.svg"), filepath.Join(directory, "prepared.svg")
	input := svgTestRead(t, "inkscape-icon.svg")
	writePNGTestFile(t, source, input)
	stamp := time.Unix(1234567890, 0)
	if err := os.Chtimes(source, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	request := SVGOptimizationRequest{Path: source, OutputPath: destination, SVGOptimizationOptions: svgTestOptions()}
	result, err := OptimizeSVG(nil, request)
	if err != nil || !result.Changed || result.OutputPath != destination || len(result.PathErrors) != 7 {
		t.Fatalf("%+v, %v", result, err)
	}
	original, _ := os.ReadFile(source)
	info, _ := os.Stat(source)
	if !bytes.Equal(input, original) || !info.ModTime().Equal(stamp) {
		t.Fatal("changed authoring source")
	}
	prepared, _ := os.ReadFile(destination)
	if result.OutputBytes != len(prepared) || result.InputBytes != len(input) || result.SavingsBytes != len(input)-len(prepared) {
		t.Fatal("inaccurate size report")
	}
	if _, err := OptimizeSVG(nil, request); err == nil {
		t.Fatal("overwrote destination")
	}
	request.OutputPath = source
	if _, err := OptimizeSVG(nil, request); err == nil {
		t.Fatal("accepted source as output")
	}
	link := filepath.Join(directory, "link.svg")
	if err := os.Link(source, link); err == nil {
		request.OutputPath = link
		if _, err := OptimizeSVG(nil, request); err == nil {
			t.Fatal("accepted source hardlink as output")
		}
	}
	request.OutputPath = filepath.Join(directory, "invalid.svg")
	writePNGTestFile(t, source, []byte(`<svg xmlns="`+svgNamespace+`"><script/></svg>`))
	if _, err := OptimizeSVG(nil, request); err == nil {
		t.Fatal("accepted invalid input")
	}
	if _, err := os.Lstat(request.OutputPath); !os.IsNotExist(err) {
		t.Fatal("published failed candidate")
	}
	// Equal-sized optimization still creates the requested copy.
	minimal := []byte(`<svg xmlns="` + svgNamespace + `"/>`)
	writePNGTestFile(t, source, minimal)
	request.SVGOptimizationOptions = SVGOptimizationOptions{}
	result, err = OptimizeSVG(nil, request)
	copy, _ := os.ReadFile(request.OutputPath)
	if err != nil || result.Changed || result.SavingsBytes != 0 || !bytes.Equal(copy, minimal) {
		t.Fatalf("copy no-op: %+v, %v", result, err)
	}
}

func svgTestRead(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "svg", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func svgTestPaths(t *testing.T, data []byte) []string {
	t.Helper()
	root, err := parseSVG(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	var visit func(*svgNode)
	visit = func(n *svgNode) {
		if n.name.Local == "path" {
			paths = append(paths, n.attribute("d"))
		}
		for _, child := range n.children {
			visit(child)
		}
	}
	visit(root)
	return paths
}

// Independent absolute-coordinate evaluator for the supplied cubic-only art.
func svgTestCubicPoints(t *testing.T, data string) [][4]svgPoint {
	t.Helper()
	segments, err := parseSVGPath(data)
	if err != nil {
		t.Fatal(err)
	}
	var current, start svgPoint
	var curves [][4]svgPoint
	for _, segment := range segments {
		origin := svgPoint{}
		if segment.command >= 'a' {
			origin = current
		}
		point := func(i int) svgPoint { return svgPoint{origin.x + segment.values[i], origin.y + segment.values[i+1]} }
		switch segment.command | 32 {
		case 'm':
			current = point(0)
			start = current
		case 'c':
			curves = append(curves, [4]svgPoint{current, point(0), point(2), point(4)})
			current = point(4)
		case 'z':
			current = start
		default:
			t.Fatalf("unexpected fixture command: %c", segment.command)
		}
	}
	return curves
}

func svgTestBezier(points [4]svgPoint, u float64) svgPoint {
	v := 1 - u
	weights := [4]float64{v * v * v, 3 * v * v * u, 3 * v * u * u, u * u * u}
	var result svgPoint
	for i, weight := range weights {
		result.x += weight * points[i].x
		result.y += weight * points[i].y
	}
	return result
}
