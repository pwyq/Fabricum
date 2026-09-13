# Configuration

Run with JSON settings:

> fabricum --config settings.json

```json
{
  "source": "images/sample.png",
  "outputDirectory": "delivery",
  "squareSize": 512,
  "wideWidth": 768,
  "encoderDirectory": "native-codecs"
}
```

## Launch behavior

- A bare launch opens the GUI and can start without `source`.
- GUI mode exits when its last Fabricum page closes.
- Without `source`, the browser prompts for a PNG, JPEG, or GIF.
- `--source path --output path` (or `-s path -o path`) automatically uses CLI
  mode, prints the editor URL, and does not open a browser.
- `--output` is an output directory and defaults to `output`.
- Both modes use the interactive editor. CLI mode is not an unattended batch mode.

## Paths

- JSON paths are relative to the config file.
- CLI paths are relative to the working directory.
- Explicit flags override JSON values.
- Unknown JSON properties and trailing JSON are rejected.
- `outputDirectory` and `--output` default to `output`.
- Outputs default to `<name>-square.webp` and `<name>-wide.webp`.
- `squareOutput` and `wideOutput` set explicit paths.
- Source and output paths must differ.

## Sizes

- `sourceSize: 0` accepts any dimensions.
- A positive `sourceSize` requires an exact square source.
- Square output defaults to 512×512.
- Wide output defaults to 768×576.
- Sources are limited to 64 megapixels.
- Outputs are limited to 8192 pixels per dimension.
- Crops must fit the source, match the output ratio, and never upscale.
- GIF input uses the first frame only.

## Commands and exit codes

- `--help` or `-h`: list flags; exit 0.
- `--version`: print the version; exit 0.
- Invalid flags or config: exit 2.
- Startup or listener failure: exit 1.

The executable embeds the editor. WebP and AVIF encoding requires the native
`cwebp` and `avifenc` tools in `encoderDirectory` or `PATH`; it does not search
for a game repository or bundle project assets.

See [project integration](integration.md) and [processing](processing.md).
