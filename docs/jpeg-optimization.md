# Optimize existing JPEG files

Run the noninteractive, safe-in-place optimizer:

```sh
fabricum optimize-jpeg image.jpg another.jpeg
fabricum optimize-jpeg --progressive image.jpg
```

Options precede paths. Use `--` before paths beginning with a dash. Paths are
relative to the current directory. Each file commits independently. Processing
is fully offline on standalone Linux x64 and Windows x64 releases. Development
builds resolve pinned `jpegtran` from `bin/codecs`, `PATH`, or an explicit
`--encoder-directory` after `npm run native:install`.

Fabricum uses libjpeg-turbo 3.1.3's coefficient-domain `jpegtran` to optimize
Huffman/entropy coding. Dimensions, quantization tables, chroma subsampling,
and quantized DCT coefficients stay intact. There is no second lossy generation,
quality setting, pixel transform, or JPEG export format in the image editor.
By default, baseline inputs remain baseline and progressive inputs remain
progressive. `--progressive` requests progressive scans; if the complete result
is not smaller, the original, including its scan mode, is retained.

Every APP0–APP15 and COM segment is copied byte for byte in its original relative
order. This includes EXIF and its orientation, ICC profile chunks, XMP including
extended XMP, IPTC/Photoshop data, JFIF/EXIF thumbnails, density, Adobe color
transform declarations, and comments. Metadata segments are collected immediately
after SOI. Orientation is never baked into the pixels; ICC data is neither
interpreted nor converted. Embedded thumbnails remain unchanged. No metadata
stripping option is provided.

Only single-frame, 8-bit, Huffman-coded baseline or progressive JPEGs with one
or three components are supported. Arithmetic, predictive lossless, hierarchical,
12-bit, CMYK/YCCK, multi-picture MPF/MPO, APP11/JUMBF/signature-bearing files,
and abbreviated Motion JPEG streams are rejected. Other unsupported sampling
layouts also fail validation. Trailing bytes and concatenated images are rejected
rather than discarded. Invalid or truncated entropy data produces an error;
native warnings are fatal. Limits are 256 MiB per input, 64 million pixels, 256 scans, and a
512 MB native memory budget. Inputs must be regular files; symlinks are rejected.

An independent Go JPEG decode validates the input without re-encoding it.
The candidate is staged away from the original. Fabricum restores metadata,
then serializes both files' DCT coefficients using the same sequential entropy
coding and checks byte equality. This also checks quantization, dimensions, and
sampling without relying on rounded RGB samples. Only a strictly smaller,
verified candidate is installed with a same-directory replacement. File mode
is retained. Equal/larger results retain the original bytes and timestamp.
Failures, cancellation, and detected concurrent edits preserve the original;
avoid editing inputs concurrently.

The command emits a schema-versioned JSON receipt to stdout:

```json
{
  "schemaVersion": 1,
  "processor": "fabricum/0.2.4",
  "files": [{
    "path": "/assets/image.jpg",
    "inputBytes": 9320,
    "outputBytes": 8210,
    "savingsBytes": 1110,
    "savingsPercent": 11.909871244635193,
    "format": "jpeg",
    "progressive": false,
    "optimizer": "jpegtran",
    "optimizerVersion": "3.1.3",
    "changed": true
  }]
}
```

The counts and `progressive` describe the retained file. Every path has a result;
failures include `error`, `changed: false`, and available measurements. Exit
status is 0 if all files succeed (including no-ops) and 1 if any fails. Diagnostics
also go to stderr. The Go host API provides `OptimizeJPEG`, `OptimizeJPEGFiles`,
and `OptimizeJPEGArgs` with the same request/result/receipt contracts.

See the [backend evaluation](jpeg-optimization-backend.md) for build, license,
size, and verification details.
