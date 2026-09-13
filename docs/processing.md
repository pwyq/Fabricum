# Processing

## Formats

- Default: WebP, quality 90.
- PNG: lossless.
- WebP and AVIF: quality 1–100 or lossless, using the pinned native codec tools.
- AVIF: 4:4:4 chroma and maximum effort; previews may be slower.
- New preview requests cancel superseded encoder work.
- PNG is deterministic with identical input, crop, and toolchain.
- WebP uses libwebp's exact-transparent-RGB option and lossless alpha.
- AVIF uses libaom, 8-bit output, 4:4:4 chroma, and lossless alpha when
  requested.
- For repeatable WebP or AVIF, use the pinned native codec versions and the
  same platform/toolchain.

## Loose KTX2 texture sets

`fabricum texture-set request.json` (also available as `texture` or `ktx2`)
builds one or more loose KTX2 files from ordinary image sources. Each output
can select `etc1s` or `uastc-zstd`, transfer function, color primaries, full
generated mipmaps, and a quality level. UASTC uses Zstandard level 6 by
default. `maxWorkers` bounds concurrent encodes; set it to `1` for a
reproducible single-worker game-asset run.

Texture outputs reuse the deterministic transform and channel-pack operations.
For an all-or-nothing material set, list `requiredRoles`, for example
`base-color`, `normal`, and `arm`; all outputs are prepared and validated
before existing delivery files can be replaced.

Texture receipts report each output's dimensions, encoding, mip count, transfer
function, color primaries, byte count, SHA-256, processor, and pinned Basis
Universal version. The KTX2 path accepts opaque RGB results; use `removeAlpha`
or RGB channel packing for source images with alpha.

## Static glTF model optimization

`fabricum gltfpack request.json` optimizes one static `.gltf` or `.glb` input
with the bundled native gltfpack build. The request remains project-neutral:
the caller supplies the output path, optional role, material names, and target
runtime compatibility choices.

```json
{
  "source": "prop.gltf",
  "output": "delivery/prop.glb",
  "compression": "meshopt",
  "textureCompression": "ktx2",
  "textureEncoding": "etc1s",
  "bakeRootTransform": true,
  "centerXZAtGround": true,
  "removeAttributes": ["COLOR_0"],
  "deduplicateMaterials": true,
  "compactBuffers": true
}
```

`compression` is `none` by default and passes gltfpack's `-noq` option so a
runtime that requires unextended geometry can opt out of meshopt compression.
Use `meshopt` to enable the preferred `EXT_meshopt_compression` output.
`textureCompression` can be `ktx2` (Basis Universal, with `etc1s` or `uastc`)
or `webp`; both texture choices require the corresponding target-runtime
extension.

Before invoking gltfpack, Fabricum validates glTF 2.0, optionally bakes a
shared root transform, repairs mirrored triangle winding, centers the XZ
footprint at the lowest Y contact, removes only caller-selected attributes,
deduplicates identical material JSON, and compacts buffer views when asked.
The output receipt includes the gltfpack/meshoptimizer tool version, output
SHA-256, bounds, geometry/material/texture/animation counts, used extensions,
native tool versions, and warnings from discarded unsupported data.

Draco is deliberately not bundled or emitted. A Draco path needs a separate
target-engine requirement and ticket.

## Deterministic transforms

The `transform` command accepts project-neutral JSON requests for crop,
exact-size fill resize, aspect-preserving contain resize, transparent padding,
alpha removal, grayscale, channel extraction, and RGB channel packing. Spatial
operations run in that order. Resize filtering supports nearest, bilinear, and
Lanczos3; the default for a new resize is Lanczos3. The built-in decoder
accepts PNG, JPEG, and GIF sources, with GIF using its first frame unless a
single-frame constraint rejects an animation.

Transform requests are fully prepared before any output is replaced. Each
output is then written through the same synchronized temporary-file replacement
used by interactive export, and its receipt measurement includes the final
dimensions, alpha presence, encoder, byte count, and SHA-256.

## File writes

- Export replaces existing outputs.
- Each output is synced to a temporary file before replacement.
- Fabricum attempts restoration if replacement fails.
- Multiple outputs and the export command are not one transaction.
- Use one writer per output set and make registration retry-safe.
- Import confirms before replacing the source with a validated PNG.

## Local server

- Binds only to loopback.
- Rejects foreign `Host` and `Origin` headers.
- Requires a random session token for mutations.
- Is intended for one trusted local user.
- Must not be exposed through a proxy or port tunnel.

General-purpose asset pipelines remain out of scope; static glTF/GLB model
optimization is limited to the operation set above.
