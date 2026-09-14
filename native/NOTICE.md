# Third-party notices

This inventory is embedded in every release executable and is available with
`fabricum third-party-notices`. Fabricum invokes embedded native command-line
tools after extracting them into a verified user cache; they are not Node
dependencies. The applicable patent-information pointers are in
[`PATENTS.md`](PATENTS.md).

| Tool and implementation | Pinned source | Included dependency | License and notice |
| --- | --- | --- | --- |
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
