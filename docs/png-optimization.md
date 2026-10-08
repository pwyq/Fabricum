# Optimize existing PNG files

Run the noninteractive optimizer without a crop, resize, or output path:

```sh
fabricum optimize-png image.png another.png
fabricum optimize-png --strip-metadata image.png
```

Put options before paths. Use `--` before paths beginning with a dash. Paths are
relative to the current directory. Existing static PNGs are optimized in place;
each file commits independently. The command runs entirely offline in standalone
Linux x64 and Windows x64 releases. Development builds can use
`--encoder-directory bin/codecs` after `npm run native:install`.

The optimizer searches DEFLATE compression and scanline filters using pinned
Oxipng. It preserves dimensions, color type, bit depth, palette, transparency,
and every decoded unpremultiplied RGBA value, including hidden RGB values of
fully transparent pixels and all 16-bit precision. It does not use the lossy
indexed PNG quantizer. Exact storage conversions are currently disabled.

By default, every non-IDAT chunk is preserved byte for byte and in order.
`--strip-metadata` explicitly removes only `tEXt`, `zTXt`, `iTXt`, and `tIME`.
ICC profiles, gamma, chromaticities, sRGB/HDR declarations, background,
significant bits, resolution, EXIF, and safe private chunks remain intact.
Profile data is preserved independently of Go's decoder, which compares sample
values without color management.

Animated PNGs, byte-dependent signatures (`dSIG`, `caBX`), Apple-specific chunks,
unknown critical chunks, and unknown chunks marked unsafe to copy are rejected.
Inputs must be regular files; symbolic links are rejected. Limits match asset
inspection: 256 MiB per input and 64 million pixels. Invalid checksums are errors,
not silently repaired.

A candidate is staged separately, decoded and checked, then installed with a
same-directory replacement only when the complete file is strictly smaller.
If it is equal or larger, the original bytes and timestamp are retained.
Failures and cancellation preserve the original. Input changes detected during
optimization abort replacement; avoid editing the file concurrently.

The command emits a versioned JSON receipt on stdout. For example:

```json
{
  "schemaVersion": 1,
  "processor": "fabricum/0.2.4",
  "files": [{
    "path": "/assets/image.png",
    "inputBytes": 9320,
    "outputBytes": 2410,
    "savingsBytes": 6910,
    "savingsPercent": 74.14163090128755,
    "format": "png",
    "colorType": "rgba",
    "bitDepth": 8,
    "optimizer": "oxipng",
    "optimizerVersion": "10.2.1",
    "changed": true,
    "metadataStripped": false
  }]
}
```

Every requested file has a result. Failed files include `error`, `changed: false`,
and any available input measurements. Exit status is 0 when all files succeed
(including unchanged files) and 1 if any file fails. Diagnostics also go to
stderr. The Go host API exposes `OptimizePNG` and `OptimizePNGFiles` with the
same request and result types. See the [backend evaluation](png-optimization-backend.md)
for pinned artifacts, licensing, size cost, and preservation decisions.
