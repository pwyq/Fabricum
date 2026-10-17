package processing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Exercise an independent SVG2 renderer, including CSS selectors and text.
// CI requires Chromium; local source-only environments may omit a browser.
func TestSVGRenderingPreserved(t *testing.T) {
	browser := svgTestBrowser(t)
	type sample struct{ Name, Before, After string }
	var samples []sample
	for _, name := range []string{"icon", "references", "text-style", "prefixed"} {
		input, err := os.ReadFile(filepath.Join("testdata", "svg", name+".svg"))
		if err != nil {
			t.Fatal(err)
		}
		output, _, err := optimizeSVGDocument(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		samples = append(samples, sample{name, base64.StdEncoding.EncodeToString(input), base64.StdEncoding.EncodeToString(output)})
	}
	data, err := json.Marshal(samples)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	page := filepath.Join(root, "render.html")
	harness := `<!doctype html><meta charset="utf-8"><pre id="result">PENDING</pre><script>
const samples = ` + string(data) + `;
async function render(data) {
  const img = new Image();
  img.src = 'data:image/svg+xml;base64,' + data;
  await img.decode();
  const canvas = document.createElement('canvas');
  canvas.width = img.naturalWidth; canvas.height = img.naturalHeight;
  const ctx = canvas.getContext('2d'); ctx.drawImage(img, 0, 0);
  return { width: canvas.width, height: canvas.height, pixels: ctx.getImageData(0, 0, canvas.width, canvas.height).data };
}
(async () => {
  const reports = [];
  for (const sample of samples) {
    const a = await render(sample.Before), b = await render(sample.After);
    if (a.width !== b.width || a.height !== b.height) throw new Error(sample.Name + ': dimensions changed');
    let painted = 0;
    for (let i = 0; i < a.pixels.length; i++) {
      if (a.pixels[i] !== b.pixels[i]) throw new Error(sample.Name + ': pixel channel changed at ' + i);
      if (i % 4 === 3 && a.pixels[i]) painted++;
    }
    if (!painted) throw new Error(sample.Name + ': fixture did not render');
    reports.push({name: sample.Name, width: a.width, height: a.height, painted});
  }
  document.getElementById('result').textContent = 'PASS ' + JSON.stringify(reports);
})().catch(error => { document.getElementById('result').textContent = 'FAIL ' + error.message; });
</script>`
	if err := os.WriteFile(page, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	pageURL := &url.URL{Scheme: "file", Path: filepath.ToSlash(page)}
	if runtime.GOOS == "windows" {
		pageURL.Path = "/" + pageURL.Path
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, browser, "--headless", "--disable-gpu", "--no-sandbox", "--disable-dev-shm-usage", "--disable-background-networking", "--disable-component-update", "--no-first-run", "--user-data-dir="+filepath.Join(root, "profile"), "--virtual-time-budget=10000", "--dump-dom", pageURL.String())
	data, err = command.CombinedOutput()
	if err != nil || !strings.Contains(string(data), `<pre id="result">PASS `) {
		t.Fatalf("SVG browser rendering failed: %v\n%s", err, data)
	}
	start := strings.Index(string(data), `<pre id="result">PASS `)
	end := strings.Index(string(data[start:]), "</pre>")
	t.Log(string(data[start : start+end]))
}

func svgTestBrowser(t *testing.T) string {
	t.Helper()
	if browser := os.Getenv("FABRICUM_SVG_BROWSER"); browser != "" {
		return browser
	}
	for _, name := range []string{"google-chrome", "chromium", "chromium-browser", "msedge"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	if runtime.GOOS == "windows" {
		for _, root := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles")} {
			path := filepath.Join(root, "Microsoft", "Edge", "Application", "msedge.exe")
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	message := fmt.Sprint("SVG rendering test requires Chromium/Edge or FABRICUM_SVG_BROWSER")
	if os.Getenv("CI") != "" {
		t.Fatal(message)
	}
	t.Skip(message)
	return ""
}
