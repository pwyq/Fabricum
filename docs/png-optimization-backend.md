# Lossless PNG backend evaluation

Fabricum selects [Oxipng 10.2.1](https://github.com/oxipng/oxipng/tree/v10.2.1).
This operation complements Go's PNG encoder and the separate lossy indexed PNG
mode. It optimizes existing static PNGs without resizing or quantization.

| Backend | Compression and filters | Distribution and licenses | Decision |
| --- | --- | --- | --- |
| [Oxipng](https://github.com/oxipng/oxipng/tree/v10.2.1) | libdeflate levels 0–12, fixed and heuristic scanline filters, optional Zopfli | Static Linux x64 musl archive; Windows GNU source build with static runtime; MIT, with dependency notices | Selected: maintained, small, strong filter search and explicit preservation controls |
| [OptiPNG](https://optipng.sourceforge.net/optipng-7.9.1.man1.html) | zlib parameter and filter trials, exact storage reductions | Portable C; zlib/libpng licenses; Linux would need an additional source build | Viable, but adds build work to the existing release pipeline |
| [ZopfliPNG](https://github.com/google/zopfli/blob/master/README.zopflipng) | Stronger, slower DEFLATE and filter search | Apache-2.0 C/C++; would require source builds; upstream repository archived | Metadata stripping defaults need extra care; Oxipng offers Zopfli without another executable |

## Packaging and cost

The official x64 archives were downloaded and checked against the GitHub release
asset digests before pinning them in `native/versions.json`:

| Platform | Archive bytes | Executable bytes |
| --- | ---: | ---: |
| Windows x64 MSVC (evaluated, rejected) | 497,302 | 991,744 |
| Linux x64 musl | 528,412 | 1,075,656 |

The Windows MSVC artifact imports `VCRUNTIME140.dll`. An empty PATH check on a
developer machine cannot establish that this redistributable is present on a
user's machine. Fabricum therefore builds Windows from its checksum-pinned
18,417,330-byte source archive with Rust 1.88.0, the
`x86_64-pc-windows-gnu` target, MinGW GCC, `cargo build --locked --release`, and
`-C target-feature=+crt-static`. CI installs the pinned MSVC host toolchain and
GNU target; only the GNU-target optimizer is distributed. The local source build
produced a 1,423,360-byte executable importing only Windows system libraries and
UCRT API sets. A Windows PE-import test rejects external runtime DLL dependencies.

The incremental compressed release payload is approximately 0.5–0.7 MiB, plus
compressed license texts. The existing release packer compresses and embeds the
tool; runtime extraction uses the existing verified content-addressed cache.
Linux uses the static musl artifact. Windows runtime independence is checked by
PE imports and the clean executable check with an empty PATH.
No Rust toolchain, download, service, or optimizer installation is needed at
runtime. Source builds may install the pinned tool with `native:install`.
Only Windows x64 and Linux x64 are supported release platforms.

Oxipng is MIT licensed; its Rust dependencies include MIT/Apache-2.0 packages,
libdeflate (MIT), and Rust Zopfli (Apache-2.0). Dependency and static runtime
notices are consolidated in `native/NOTICE.md` and embedded in releases.
MinGW CRT includes ZPL/BSD terms; GCC runtime redistribution uses its runtime
exception.

## Preservation contract

Use level 6, all preset filter trials, libdeflate level 12, one thread, and
`--nx --interlace keep`. Do not enable alpha optimization, lossy 16-bit scaling,
checksum repair, palette quantization, or storage reductions. Exact storage
conversion is deferred: unchanged color type, bit depth, palette, transparency,
and rendering chunks give a stronger and simpler contract for profiled assets.

Oxipng processes a disposable copy with forced output so Fabricum can validate
even a larger candidate. Fabricum uses only its IDAT stream, copying every other
original chunk verbatim and in order. This prevents incidental profile
recompression or metadata stripping by the backend. The candidate must decode
to the same unpremultiplied 16-bit RGBA values, including RGB beneath zero alpha.
The original is replaced only when the complete validated candidate is smaller.

Metadata stripping is opt-in and limited to `tEXt`, `zTXt`, `iTXt`, and `tIME`.
Color profiles, gamma, chromaticities, HDR information, significant bits,
background, resolution, and EXIF remain untouched. Reject animation chunks,
digital/content signatures, Apple-specific PNG variants, and unknown chunks
marked unsafe to copy; their meaning may depend on the original byte stream.
No-change outcomes preserve bytes and timestamps. Invalid inputs, failed tools,
invalid candidates, canceled work, and failed replacement leave the original
in place. Per-file JSON measurements and diagnostics make batch behavior explicit.
