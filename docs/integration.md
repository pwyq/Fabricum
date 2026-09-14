# Project integration

Use `sources` to restrict the editor to project-owned inputs:

```json
{
  "sourceSize": 1024,
  "sources": [
    {
      "path": "images/sample.png",
      "squareOutput": "delivery/sample-square.webp",
      "wideOutput": "delivery/sample-wide.webp"
    }
  ],
  "exportCommand": ["node", "register-export.mjs"]
}
```

## Sources

- The editor only shows configured sources.
- Source lists load at startup. Restart after changes.
- Each source may set its square and wide output paths.

## Export command

- Runs after output files are written.
- Receives a version 1 JSON receipt on stdin. Version 1 is additive: ignore
  unknown JSON properties and treat a new `encoder` value as data, not policy.
- Runs as an argument array without a shell.
- Uses the config directory as its working directory.
- Times out after 30 seconds.
- May run more than once; make it idempotent.
- A failure leaves the output files in place and fails the export response.
- Prefer an absolute executable path.
- Only use trusted config files. The command runs with your permissions.

The receipt contains:

- `processor` and absolute `source`.
- `request`: crops, format, quality, and lossless mode.
- `outputs`: role, path, dimensions, format, actual encoder, bytes, SHA-256,
  and crop. Current encoder values are `go/image/png`, `cwebp/libwebp`, and
  `avifenc/libavif+libaom`.

Keep project naming, manifests, ownership, and alternate-format cleanup in the
export command. Fabricum does not remove alternate formats.

## Noninteractive transforms

`fabricum transform request.json` performs the same project-neutral processing
without starting the browser. It writes a version 1 JSON receipt to stdout;
the receipt contains the absolute `source`, the original `request`, and one
measurement per output. Relative source, output, channel-source, and codec
paths are resolved from the request file directory.

See [configuration](configuration.md) for path rules and CLI overrides.

## Loose KTX2 texture sets

Use a texture-set request when a project needs independently loaded texture
files rather than textures embedded in a model. The request is project-neutral:
each output supplies its own source, delivery path, transform, and KTX2
encoding options.

```json
{
  "maxWorkers": 1,
  "requiredRoles": ["base-color", "normal", "arm"],
  "outputs": [
    {
      "role": "base-color",
      "source": "base.png",
      "path": "delivery/base.ktx2",
      "encoding": {
        "encoding": "etc1s",
        "mipLevels": 11,
        "transferFunction": "srgb",
        "colorPrimaries": "bt709"
      }
    },
    {
      "role": "normal",
      "source": "normal.png",
      "path": "delivery/normal.ktx2",
      "encoding": {
        "encoding": "uastc-zstd",
        "mipLevels": 11,
        "transferFunction": "linear",
        "colorPrimaries": "unspecified"
      }
    },
    {
      "role": "arm",
      "source": "base.png",
      "path": "delivery/arm.ktx2",
      "transform": {
        "pack": {
          "red": {"source": "ao.png", "channel": "red"},
          "green": {"source": "roughness.png", "channel": "red"},
          "blue": {"constant": 0}
        }
      },
      "encoding": {
        "encoding": "uastc-zstd",
        "mipLevels": 11,
        "transferFunction": "linear",
        "colorPrimaries": "unspecified"
      }
    }
  ]
}
```

All outputs are prepared before any delivery replacement. A missing required
role, source channel, invalid color contract, or native encoder failure leaves
existing deliveries untouched. The receipt is schema version 1 and includes
the request plus output encoding, dimensions, mip count, transfer function,
color primaries, byte count, SHA-256, processor, and native encoder versions.

## Static model optimization

Use `fabricum gltfpack request.json` for a static glTF/GLB build. Relative
source, output, and native-tool paths are resolved from the request file
directory; glTF resource paths retain glTF semantics and are resolved from
the model file. The output path and any material names are caller data;
Fabricum does not assign project-specific semantic names.

Set `compression` to `meshopt` for meshoptimizer compression, or to `none`
when the target loader requires ordinary unextended geometry. Set
`textureCompression` to `ktx2` or `webp` only when the target loader supports
the corresponding glTF extension. Release binaries extract their embedded
gltfpack into a content-addressed user cache. Source builds use an explicitly
configured directory, a `codecs` directory beside Fabricum, or `PATH`; Node
and an npm-installed gltfpack are not involved.

The model receipt is schema version 1. It reports the Fabricum processor,
gltfpack tool version, request, every main/sidecar output's byte count and
SHA-256, bounds, triangle/primitive/material/texture/animation counts, used
extensions, native tool versions, and warnings. Unsupported input extensions
are surfaced as warnings rather than silently presented as supported output.
