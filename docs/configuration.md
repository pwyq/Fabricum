# Configuration

```json
{
  "mode": "gui",
  "source": "images/sample.png",
  "outputDirectory": "delivery",
  "squareSize": 512,
  "wideWidth": 768,
  "encoderDirectory": "."
}
```

The executable defaults to GUI mode. GUI mode can start without `source`; the
browser then prompts for a PNG, JPEG, or GIF upload and keeps the temporary
source in a local session directory. `-mode gui` makes this explicit. CLI mode
is path-driven, requires `source` (or `-source`), prints the local editor URL,
and does not open a browser. Select it with `"mode": "cli"` or `-mode cli`.

Run `fabricum -config settings.json`. Paths in JSON resolve relative to the JSON
file; CLI paths resolve relative to the working directory. Explicit flags override
JSON values. Unknown properties and trailing JSON are rejected. `sourceSize: 0`
accepts arbitrary dimensions; a positive value requires an exact square source.

Default outputs are `<source-name>-square.webp` and `<source-name>-wide.webp` in
`output`. The selected format changes the extension. `squareOutput` and
`wideOutput` (or their CLI flags) supply explicit paths. Input and output paths
must be distinct. Square output defaults to 512×512, wide output to 768×576.
Inputs may be PNG, JPEG, or GIF; only the first GIF frame is processed. Inputs are
limited to 64 megapixels and outputs to 8192 pixels per dimension. Crops must fit
the image, match the role ratio, and never upscale.

`-help` lists options; `-version` prints the version. Help/version exit 0, malformed
CLI/configuration exits 2, and startup/listener errors exit 1. Both modes start
the interactive local editor; CLI mode does not implement unattended batch crop
selection.

The executable embeds the editor and encoder script; it never searches for a
game repository. No image fixtures or project assets are bundled.

See [project integration](integration.md) for source lists and export commands,
and [processing behavior](processing.md) for encoding and write details.
