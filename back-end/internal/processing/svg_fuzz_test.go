package processing

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func FuzzSVGOptimization(f *testing.F) {
	for _, name := range []string{"icon", "references", "text-style", "prefixed"} {
		data, err := os.ReadFile(filepath.Join("testdata", "svg", name+".svg"))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	for _, input := range []string{
		`<svg xmlns="http://www.w3.org/2000/svg"><g a = 'b' /></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><style><![CDATA[rect{fill:red}]]></style></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><g><rect/><circle/></g><path/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xml="bad"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" x="unterminated>`,
	} {
		f.Add([]byte(input))
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > 65536 {
			t.Skip()
		}
		output, _, err := optimizeSVGDocument(context.Background(), input)
		if err != nil {
			return
		}
		repeated, _, err := optimizeSVGDocument(context.Background(), output)
		if err != nil || !bytes.Equal(output, repeated) {
			t.Fatalf("accepted output was invalid or unstable: %s -> %s (%v)", output, repeated, err)
		}
	})
}
