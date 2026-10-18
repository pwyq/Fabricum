package processing

import (
	"context"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestSVGPathSerialization(t *testing.T) {
	for _, data := range []string{
		"M 0,0 10,20 30,40 L 50,60 l-1-.2 H1h2V3v4z m1 1Z",
		"M.1-.2 C1 2 3 4 5 6c-1 -2-3-4-5-6S1 2 3 4s1 2 3 4Q1 2 3 4q1 2 3 4T1 2t3 4Z",
		"M1e-3 2E+3 A10 20 30 011.5 2a1 2 3 10-4-5",
		"M+000.1000 -.000L.1234567890123456789 1234567890123456789",
	} {
		segments, err := parseSVGPath(data)
		if err != nil {
			t.Fatalf("%s: %v", data, err)
		}
		output := writeSVGPath(segments)
		reparsed, err := parseSVGPath(output)
		if err != nil || len(segments) != len(reparsed) {
			t.Fatalf("invalid serialization: %s, %v", output, err)
		}
		for i, segment := range segments {
			if segment.command != reparsed[i].command || !reflect.DeepEqual(segment.values, reparsed[i].values) {
				t.Fatalf("changed path: %s -> %s", data, output)
			}
		}
		if again := writeSVGPath(reparsed); again != output {
			t.Fatalf("unstable: %s -> %s", output, again)
		}
		if strings.Contains(data, "1234567890123456789") && !strings.Contains(output, "1234567890123456789") {
			t.Fatal("rounded without permission")
		}
	}
	for _, data := range []string{"L1 2", "M", "M0", "M0 0,", "M,0 0", "M0,,0", "M0 0L", "M0 0Z1 2", "M0 0R1 2", "M0 0A1 1 0 2 0 2 2", "M0 0A-1 1 0 0 1 2 2", "M1e 2", "MNaN 0", "M1e999 0"} {
		if _, err := parseSVGPath(data); err == nil {
			t.Fatalf("accepted malformed path %q", data)
		}
	}
}

func TestSVGPathAccumulatedError(t *testing.T) {
	input := `<svg xmlns="` + svgNamespace + `"><path d="M0 0` + strings.Repeat("l.011 .011", 100) + `"/></svg>`
	_, _, reports, err := optimizeSVGWithOptions(context.Background(), []byte(input), SVGOptimizationOptions{SelectiveRounding: true})
	want := math.Hypot(100*.011, 100*.011)
	if err != nil || len(reports) != 1 || math.Abs(reports[0].MaxDeviation-want) > 1e-12 {
		t.Fatalf("relative accumulation: %+v, %v, want %v", reports, err, want)
	}
	// Smooth control reflection can exceed both adjacent endpoint errors.
	for name, test := range map[string]struct {
		path string
		want float64
	}{
		"cubic-reflection":     {"M0 0C.01 0 .09 0 0 0S0 0 0 0", .01},
		"quadratic-reflection": {"M0 0Q0 0 .01 0T0 0", .02},
		"absolute-reset":       {"M0 0l.01 .01L0 0l.01 .01", math.Sqrt2 * .01},
		"close-reset":          {"M0 0l.01 .01zm0 0l.01 .01", math.Sqrt2 * .01},
		"horizontal-vertical":  {"M0 0h.01v.01H0V0", math.Sqrt2 * .01},
		"move-sequence":        {"m.01 .01 .01 .01 .01 .01", math.Sqrt2 * .03},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, reports, err := optimizeSVGWithOptions(context.Background(), []byte(`<svg xmlns="`+svgNamespace+`"><path d="`+test.path+`"/></svg>`), SVGOptimizationOptions{SelectiveRounding: true})
			if err != nil || len(reports) != 1 || math.Abs(reports[0].MaxDeviation-test.want) > 1e-12 {
				t.Fatalf("%+v, %v, want %v", reports, err, test.want)
			}
		})
	}
}

func TestSVGArcPathsKeepPrecision(t *testing.T) {
	input := []byte(`<svg xmlns="` + svgNamespace + `"><path d="M.999 .01l.99 .89A1.123 2.456 45.99 01.21 .11"/></svg>`)
	output, _, reports, err := optimizeSVGWithOptions(context.Background(), input, svgTestOptions())
	if err != nil || len(reports) != 1 || reports[0].MaxDeviation != 0 || reports[0].PreservedReason == "" {
		t.Fatalf("%+v, %v", reports, err)
	}
	before, _ := parseSVGPath(svgTestPaths(t, input)[0])
	after, _ := parseSVGPath(svgTestPaths(t, output)[0])
	for i := range before {
		if !reflect.DeepEqual(before[i].values, after[i].values) {
			t.Fatal("rounded arc path")
		}
	}
}

func TestSVGRoundingRespectsDecimalSpelling(t *testing.T) {
	options := SVGOptimizationOptions{SelectiveRounding: true}
	for input, want := range map[string]string{".11000000000000000001": ".11000000000000000001", ".110": "0.1", "1.10e-1": "0.1", "1.1234567890123456789": "1.1234567890123456789"} {
		segments, err := parseSVGPath("M" + input + " 0")
		if err != nil {
			t.Fatal(err)
		}
		got, _ := roundSVGNumber(input, segments[0].values[0], options)
		if got != want {
			t.Fatalf("round(%s) = %s, want %s", input, got, want)
		}
	}
	precision := 2
	got, _ := roundSVGNumber("1.00000000000000001", 1, SVGOptimizationOptions{Precision: &precision})
	if got != "1" {
		t.Fatalf("precision did not reduce a long decimal: %s", got)
	}
}

func TestSVGIconNamespaceShadowingAndMetadataReferences(t *testing.T) {
	input := []byte(`<s:svg xmlns:s="` + svgNamespace + `" xmlns:x="unused" xmlns:i="http://www.inkscape.org/namespaces/inkscape"><s:metadata id="credit">Credit</s:metadata><s:g xmlns:x="http://www.w3.org/1999/xlink"><s:defs><s:path id="shape" d="M0 0L1 1"/></s:defs><s:use x:href="#shape" aria-describedby="credit"/></s:g><i:namedview id="unused"/></s:svg>`)
	output, _, _, err := optimizeSVGWithOptions(context.Background(), input, SVGOptimizationOptions{Icon: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{`xmlns:s=`, `xmlns:x="http://www.w3.org/1999/xlink"`, `x:href="#shape"`, `id="credit"`} {
		if !strings.Contains(string(output), kept) {
			t.Fatalf("lost namespace/reference: %s", output)
		}
	}
	for _, removed := range []string{`xmlns:i=`, `xmlns:x="unused"`, `namedview`} {
		if strings.Contains(string(output), removed) {
			t.Fatalf("retained editor/unused namespace: %s", output)
		}
	}
}

func TestSVGIconPreservesUseAndMarkerInheritance(t *testing.T) {
	input := []byte(`<svg xmlns="` + svgNamespace + `"><defs><g id="shape" fill="black"><path stroke-width="7" d="M0 0L1 1"/></g><marker id="mark"><path d="M0 0L1 1"/></marker></defs><use href="#shape" fill="red" stroke="blue"/><g marker-end="url(#mark)"><path stroke-width="9" d="M0 0L2 2"/></g></svg>`)
	output, _, _, err := optimizeSVGWithOptions(context.Background(), input, SVGOptimizationOptions{Icon: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{`id="shape" fill="black"`, `stroke-width="7"`, `stroke-width="9"`, `marker-end="url(#mark)"`} {
		if !strings.Contains(string(output), kept) {
			t.Fatalf("changed inherited use/marker paints: %s", output)
		}
	}
}

func FuzzSVGIconOptimization(f *testing.F) {
	for _, data := range []string{`<svg xmlns="` + svgNamespace + `" viewBox="-.6 .5 23.4 24.6"><path stroke="black" stroke-width="11.5295" d="M0 0c.99 .999 .1 .01 .12 .21z"/></svg>`, `<svg xmlns="` + svgNamespace + `"><path style="fill:red;stroke:black" d="M1 2A1 1 0 013 4"/></svg>`} {
		f.Add([]byte(data))
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > 65536 {
			t.Skip()
		}
		output, _, _, err := optimizeSVGWithOptions(context.Background(), input, svgTestOptions())
		if err != nil {
			return
		}
		again, _, _, err := optimizeSVGWithOptions(context.Background(), output, svgTestOptions())
		if err != nil || string(output) != string(again) {
			t.Fatalf("unstable: %s -> %s (%v)", output, again, err)
		}
	})
}
