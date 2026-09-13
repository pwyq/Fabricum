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

3D and general-purpose asset pipelines are out of scope.
