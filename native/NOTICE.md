# Third-party notices and patent information

This inventory is embedded in every release executable and is available with
`fabricum third-party-notices`. Fabricum invokes embedded native command-line
tools after extracting them into a verified user cache; they are not Node
dependencies. The applicable license notices and patent-information pointers
are listed below.

The built-in **fabricum-svg 1.1.0** optimizer is Fabricum's own Apache-2.0 code,
compiled with the Go standard library/runtime (BSD-3-Clause). It adds no SVG
packages, external executables, JavaScript runtime, or transitive dependencies.
Its version is pinned in the source and in every SVG size result; SVGO and
svgcleaner are not bundled.

| Tool and implementation | Pinned source | Included dependency | License and notice |
| --- | --- | --- | --- |
| `jpegtran` / libjpeg-turbo 3.1.3 | [libjpeg-turbo 3.1.3](https://github.com/libjpeg-turbo/libjpeg-turbo/tree/3.1.3) | Static libjpeg API library, static Windows CRT; SIMD and TurboJPEG API disabled | IJG and BSD-3-Clause; full upstream license and IJG README in [licenses/libjpeg-turbo.txt](licenses/libjpeg-turbo.txt), embedded in releases |
| `oxipng` 10.2.1 | [Oxipng v10.2.1](https://github.com/oxipng/oxipng/tree/v10.2.1) | libdeflate and Rust Zopfli, locked Rust dependencies, and static platform runtimes | MIT; dependency and static runtime notices are consolidated in this inventory |
| `cwebp` / libwebp 1.6.0 | [libwebp v1.6.0](https://github.com/webmproject/libwebp/tree/v1.6.0) | libwebp and libsharpyuv | [COPYING](https://github.com/webmproject/libwebp/blob/v1.6.0/COPYING) |
| `avifenc` / libavif 1.4.2 | [libavif v1.4.2](https://github.com/AOMediaCodec/libavif/tree/v1.4.2) | libaom 3.14.1 (`aom` only) | [libavif LICENSE](https://github.com/AOMediaCodec/libavif/blob/v1.4.2/LICENSE), [libaom LICENSE](https://aomedia.googlesource.com/aom/+/v3.14.1/LICENSE) |
| `basisu` / Basis Universal 2.0.3 (`21fb6e2`) | [Basis Universal v2_0_3](https://github.com/BinomialLLC/basis_universal/tree/v2_0_3) | Basis Universal encoder and bundled Zstandard | [LICENSE](https://github.com/BinomialLLC/basis_universal/blob/v2_0_3/LICENSE), [NOTICE](https://github.com/BinomialLLC/basis_universal/blob/v2_0_3/NOTICE) |
| `gltfpack` / meshoptimizer 1.2 | [meshoptimizer v1.2](https://github.com/zeux/meshoptimizer/tree/v1.2) | meshoptimizer, Basis Universal 2.0.3, libwebp 1.6.0, cgltf, fast_obj, and sdefl | [meshoptimizer LICENSE.md](https://github.com/zeux/meshoptimizer/blob/v1.2/LICENSE.md), plus the Basis Universal and libwebp notices above |

The libavif build must enable only its `aom` encoder and must not silently
select rav1e, SVT-AV1, or another AV1 implementation. Keep the upstream
license and patent notices for every native artifact distributed with a
release. Basis Universal is built with its in-tree Zstandard implementation;
keep its upstream notice with the executable. The exact URLs, versions,
source digests, artifact digests, and build inputs are recorded in
[`versions.json`](versions.json).

The gltfpack source distribution also contains the single-header `cgltf`,
`fast_obj`, and `sdefl` dependencies used by its glTF reader and fallback
compression code. Preserve their upstream license text shipped with the
meshoptimizer source when redistributing the bundled executable.

Oxipng's dependencies include libdeflate (MIT), Rust Zopfli (Apache-2.0), and
Rust packages under MIT/Apache-2.0. Static runtime notices cover Rust
(MIT/Apache-2.0), musl (MIT), MinGW CRT (ZPL/BSD), and the GCC runtime exception.

## Patent information

This software is based in part on the work of the Independent JPEG Group.
The bundled JPEG tool retains the [libjpeg-turbo license](licenses/libjpeg-turbo.txt)
and IJG documentation; it is used only for coefficient-preserving Huffman coding.

This section records the upstream patent-information files that apply to the
bundled codec implementations. It is an attribution pointer, not legal
advice; review the pinned upstream files before each distribution.

| Implementation | Applicable information |
| --- | --- |
| Oxipng 10.2.1 | MIT; Rust Zopfli (Apache-2.0) and Rust dependencies under MIT/Apache-2.0 retain their respective license terms; no separate patent file is declared by Oxipng |
| libwebp 1.6.0 | [PATENTS](https://github.com/webmproject/libwebp/blob/v1.6.0/PATENTS) and the [COPYING](https://github.com/webmproject/libwebp/blob/v1.6.0/COPYING) terms |
| libavif 1.4.2 | [libavif LICENSE](https://github.com/AOMediaCodec/libavif/blob/v1.4.2/LICENSE); AV1 encoder terms come from libaom below |
| libaom 3.14.1 | [PATENTS](https://aomedia.googlesource.com/aom/+/v3.14.1/PATENTS), [LICENSE](https://aomedia.googlesource.com/aom/+/v3.14.1/LICENSE), and the [AOMedia patent license](https://aomedia.org/license/patent-license/) |
| Basis Universal 2.0.3 | [Basis Universal LICENSE](https://github.com/BinomialLLC/basis_universal/blob/v2_0_3/LICENSE) and [NOTICE](https://github.com/BinomialLLC/basis_universal/blob/v2_0_3/NOTICE); no separate patent file is declared by the pinned source |
| meshoptimizer 1.2 | [meshoptimizer LICENSE.md](https://github.com/zeux/meshoptimizer/blob/v1.2/LICENSE.md); its bundled codec dependencies retain the notices listed above |
