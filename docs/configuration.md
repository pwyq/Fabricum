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
- `fabricum transform request.json` runs a noninteractive transform and writes
  its versioned receipt as JSON to stdout. Use `-` instead of a request path to
  read JSON from stdin; relative paths are resolved from the request file.
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

## Asset inspection

Use `fabricum inspect path [path ...]` (or `fabricum --inspect path ...`) for a
bounded, noninteractive JSON report. Results use schema version 1, are emitted
in input order, and contain project-neutral file facts under `assets`. The
invocation accepts at most 1024 paths; each file is limited to 256 MiB and
each failure message is limited to 512 bytes. A report is written even when a
file is missing, malformed, mismatched with its extension, or unsupported; the
process exits 1 if any result contains `error`.

Inspection never calculates content hashes and never writes its inputs.

## Noninteractive transforms

Transform requests are project-neutral JSON descriptions of one source and one
or more outputs. For example:

```json
{
  "source": "images/hero.png",
  "constraints": {
    "format": "png",
    "singleFrame": true,
    "hasAlpha": true,
    "width": 1024,
    "height": 1024
  },
  "format": "png",
  "outputs": [
    {
      "role": "sprite",
      "path": "delivery/hero.png",
      "transform": {
        "resize": {
          "width": 256,
          "height": 256,
          "fit": "contain",
          "filter": "lanczos3"
        }
      }
    }
  ]
}
```

Spatial operations run as crop, resize, then transparent padding. Resize
supports `fill` and aspect-preserving `contain`; filters are `nearest`,
`bilinear`, and `lanczos3`. Color operations support `removeAlpha`,
`grayscale`, a selected `channel` (`red`, `green`, `blue`, `alpha`, or
`gray`), and generic RGB `pack` inputs. A pack input can read a channel from
another image with `source` or provide a byte `constant`, so channels such as
AO, roughness, and zero can be described without a project-specific command.

PNG transforms use deterministic lossless encoding. Transparent outputs retain
alpha; removing alpha or packing channels produces opaque RGB data. Supported
source formats are PNG, JPEG, and GIF; GIF transforms use the first frame, or
fail when `singleFrame` is requested for an animation. Transform outputs are
written atomically and report dimensions, format, filter, alpha presence,
encoder, byte count, and SHA-256 in the receipt.

The executable embeds the editor. WebP and AVIF encoding requires the native
`cwebp` and `avifenc` tools in `encoderDirectory` or `PATH`; it does not search
for a game repository or bundle project assets.

See [project integration](integration.md) and [processing](processing.md).
