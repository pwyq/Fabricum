package processing

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSVGPrecisionRoundsStrokeAndIntegerViewBox(t *testing.T) {
	input := []byte(`<svg xmlns="` + svgNamespace + `" viewBox="-.6 .5 23.4 24.6" width="23.456" height="24.567" stroke-width="11.5295" transform="translate(.999 .12345)"><path stroke="black" stroke-width=".019" opacity=".999" d="M.999 .12345L2 3"/><g stroke-width="1.2345px"><path stroke="black" stroke-width=".011%" d="M0 0L1 1"/></g><svg viewBox="0,0,2.54e1,2.46e1"/></svg>`)
	precision := 2
	options := SVGOptimizationOptions{Precision: &precision, SelectiveRounding: true}
	output, _, _, err := optimizeSVGWithOptions(context.Background(), input, options)
	if err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{
		`viewBox="-1 1 23 25"`, `viewBox="0 0 25 25"`, `stroke-width="11.53"`,
		`stroke-width="0.02"`, `stroke-width="1.23px"`, `stroke-width="0.01%"`,
		`d="M1 .12L2 3"`, `width="23.456"`, `height="24.567"`,
		`transform="translate(.999 .12345)"`, `opacity=".999"`,
	} {
		if !bytes.Contains(output, []byte(kept)) {
			t.Fatalf("missing %s in %s", kept, output)
		}
	}
	again, _, _, err := optimizeSVGWithOptions(context.Background(), output, options)
	if err != nil || !bytes.Equal(output, again) {
		t.Fatalf("unstable numeric rounding: %s -> %s (%v)", output, again, err)
	}
	// Viewports use integers even when path and stroke precision is finer.
	precision = 8
	output, _, _, err = optimizeSVGWithOptions(context.Background(), input, options)
	if err != nil || !bytes.Contains(output, []byte(`viewBox="-1 1 23 25"`)) || !bytes.Contains(output, []byte(`stroke-width="11.5295"`)) {
		t.Fatalf("viewBox followed decimal precision: %s (%v)", output, err)
	}
}

func TestSVGDefaultAndSelectiveOnlyPreservePresentationPrecision(t *testing.T) {
	input := []byte(`<svg xmlns="` + svgNamespace + `" viewBox="0 0 253.99999 254"><path stroke-width="11.5295" stroke="black" d="M.99 .89L2 3"/></svg>`)
	for _, options := range []SVGOptimizationOptions{{}, {Icon: true}, {SelectiveRounding: true}} {
		output, _, _, err := optimizeSVGWithOptions(context.Background(), input, options)
		if err != nil || !bytes.Contains(output, []byte(`viewBox="0 0 253.99999 254"`)) || !bytes.Contains(output, []byte(`stroke-width="11.5295"`)) {
			t.Fatalf("unexpected presentation rounding: %s (%v)", output, err)
		}
	}
}

func TestSVGPrecisionRoundsInlineAndReferencedStrokes(t *testing.T) {
	input := []byte(`<svg xmlns="` + svgNamespace + `" viewBox="0 0 24.123 23.876"><defs><path id="shape" style="stroke:black;stroke-width:11.5295;fill:none" d="M0 0L1 1"/></defs><use href="#shape"/><path stroke="black" stroke-width="calc(1 + .12345)" d="M0 0L1 1"/></svg>`)
	precision := 2
	output, _, _, err := optimizeSVGWithOptions(context.Background(), input, SVGOptimizationOptions{Icon: true, Precision: &precision})
	if err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{`viewBox="0 0 24 24"`, `id="shape"`, `href="#shape"`, `stroke-width="11.53"`, `fill="none"`, `stroke-width="calc(1 + .12345)"`} {
		if !bytes.Contains(output, []byte(kept)) {
			t.Fatalf("lost reference/stroke semantics: %s", output)
		}
	}
}

func TestSVGIntegerViewBoxRejectsInvalidOrCollapsedDimensions(t *testing.T) {
	precision := 2
	for _, viewBox := range []string{"0 0 .49 24", "0 0 24 .49", "0 0 -1 24", "0 0 24", "0 0 24 24 24", "0 0, ,24 24", "0 0 NaN 24", "0 0 1e999 24"} {
		path := filepath.Join(t.TempDir(), "source.svg")
		input := []byte(`<svg xmlns="` + svgNamespace + `" viewBox="` + viewBox + `"><!-- leave source intact --></svg>`)
		if err := os.WriteFile(path, input, 0600); err != nil {
			t.Fatal(err)
		}
		result, err := OptimizeSVG(nil, SVGOptimizationRequest{Path: path, SVGOptimizationOptions: SVGOptimizationOptions{Precision: &precision}})
		if err == nil || result.Changed {
			t.Fatalf("accepted invalid/collapsed viewBox %q", viewBox)
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(input, after) {
			t.Fatal("failed rounding changed source")
		}
	}
}
