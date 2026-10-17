# JPEG optimization backend evaluation

## Decision

Use libjpeg-turbo 3.1.3's `jpegtran`, built from the official release source
tarball on Linux x64 and Windows x64. [Upstream documentation](https://github.com/libjpeg-turbo/libjpeg-turbo/blob/3.1.3/doc/jpegtran.1)
describes coefficient-domain transcoding and Huffman/progressive options.
It also documents the default metadata copying behavior, which is insufficient
for Fabricum's preservation policy.

The selected command is `jpegtran -strict -copy none -optimize -maxscans 256
-maxmemory 512M`, with `-progressive` when requested or already present. Only
entropy representation changes. Quality, subsampling, rotation, crop, grayscale,
and arithmetic options are never passed. Metadata is restored by Fabricum from
the source, instead of relying on `-copy all` to retain JFIF thumbnail bytes.

`jpegoptim` was considered as a file-oriented wrapper around libjpeg. `jpegtran`
provides the smaller integration surface here: an explicit staged output and
coefficient-only options, while Fabricum owns size comparisons, metadata policy,
atomic replacement, batch errors, and receipts. No shell or runtime downloads
are needed. Re-encoding through the ordinary image-export pipeline would lose
the original quantized DCT coefficients and is outside this operation.

## Preservation and proof

Supported inputs are validated across the complete stream, including metadata
between progressive scans, and decoded independently with Go before optimization.
This rejects abbreviated Motion JPEG inputs with missing Huffman tables, for
which libjpeg otherwise silently supplies defaults even in strict mode.
MPF/MPO and APP11/JUMBF containers are refused because
rewriting entropy data could invalidate external offsets or signatures. Other
APPn and COM bytes are retained in order immediately after SOI, including ICC,
EXIF orientation, XMP, and thumbnails. Color profiles are opaque data; decoded
samples alone cannot verify their intended display semantics.

For each candidate, the original and assembled result are separately transcoded
to deterministic sequential Huffman streams with metadata copying disabled.
Their bytes must match exactly. This compares the serialized coefficients and
their quantization/sampling/dimensions, including coefficient differences that
round to the same displayed pixels. The pinned native implementation is the
trusted coefficient reader; native warnings are fatal on all three passes.

Generated grayscale and color fixtures cover baseline and progressive encoding,
odd dimensions/partial MCU edges, an ICC RGB matrix profile split over APP2
segments, EXIF orientation 6, XMP, a JFIF thumbnail, and comments. Tests compare
canonical coefficient streams, exact metadata bytes, and independently decoded
Go JPEG samples. Progressive fixtures include legal marker fill to demonstrate
reduction even when their entropy coding is already optimized. Mock native-tool
tests reject a second lossy JPEG generation and cover version mismatch, missing
or invalid output, process failure, and cancellation. Replacement tests cover
growth/no-ops, concurrent edits, and Windows file locks and runtime DLL imports.

## Distribution

The official source tarball and SHA-256 are pinned in `native/versions.json`.
The [upstream build instructions](https://github.com/libjpeg-turbo/libjpeg-turbo/blob/3.1.3/BUILDING.md)
define the static library/tool and CRT options. The installer builds
`jpegtran-static` with shared libraries, TurboJPEG API, and SIMD disabled. SIMD
does not accelerate this coefficient-only path; disabling it avoids requiring
NASM. Windows uses the static MSVC CRT (`WITH_CRT_DLL=OFF`). Linux links libjpeg
statically and uses system libc. No new runtime sidecar library is required.

A local Windows x64 Release build measured 465,408 bytes uncompressed and
171,691 bytes with ZIP-compatible DEFLATE (about 168 KiB payload cost). Actual
release size depends on the platform/compiler and includes Go integration and
license text. Existing release size gates remain in force. The standalone
compatibility check optimizes a synthetic JPEG with an empty `PATH` and proves
an unchanged second run.

This software is based in part on the work of the Independent JPEG Group.
The IJG and libjpeg-turbo BSD license text is retained in
`native/licenses/libjpeg-turbo.txt`, embedded in standalone releases, and printed
by `fabricum third-party-notices`. See [native notices](../native/NOTICE.md).
