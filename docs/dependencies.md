# Dependencies

## Runtime and build tools

| Dependency | Purpose | License notes |
| --- | --- | --- |
| Go 1.26.6 | Server, image processing, PNG, hashing, files, and tests. No third-party Go modules. | BSD-3-Clause. Preserve the Go license when redistributing binaries. |
| `cwebp` from libwebp 1.6.0 | WebP preview and export encoding. | BSD-3-Clause and libsharpyuv notice. |
| `avifenc` from libavif 1.4.2 | AVIF preview and export container encoding. | BSD license and upstream notices. |
| libaom 3.14.1 | The only AV1 encoder enabled for libavif. | AOMedia license and patent notice. |
| `basisu` from Basis Universal 2.0.3 | Loose ETC1S and UASTC-Zstandard KTX2 encoding and mipmap generation. | Apache-2.0 and upstream Basis Universal notice; bundled Zstandard is covered by the upstream source notice. |
| `gltfpack` from meshoptimizer 1.2 | Static model optimization, optional EXT_meshopt compression, Basis KTX2 texture compression, and WebP texture compression. | MIT meshoptimizer license plus the bundled Basis Universal, libwebp, cgltf, fast_obj, and sdefl notices. |
| Node.js >=24.15 | Repository scripts, frontend tests, and optional export hooks. It is not used by image encoding. | Node and hook-owned notices apply. Not required by the encoder path. |

The native versions, URLs, Windows artifact digest, selected AV1 codec, and
Linux build flags are pinned in [`../native/versions.json`](../native/versions.json).
Required attributions and upstream notice links are in
[`../native/NOTICE.md`](../native/NOTICE.md). Review the upstream license and
patent terms again before distributing native binaries.

## Native build inputs

- `cwebp` is built from or taken from the official libwebp 1.6.0 release.
- `avifenc` is built from libavif 1.4.2 with `AVIF_CODEC_AOM=LOCAL`; libavif's
  pinned local dependency is libaom 3.14.1.
- libavif is invoked with the `aom` codec explicitly. rav1e, SVT-AV1, and
  decoder-only codecs are not part of Fabricum's encoding contract.
- `basisu` is built from the pinned Basis Universal 2.0.3 source archive with
  its in-tree Zstandard implementation. The KTX2 path invokes it with
  `-etc1s` or `-uastc -ktx2_zstandard_level 6`, generated mipmaps, and no
  internal multithreading.
- The command-line integration uses dynamically discoverable native tools. The
  native installer places the statically linked gltfpack build beside the
  other tools in `bin/codecs`; static CGO integration remains deferred.

## Distribution

- Source archives include all Git-tracked project files, including the lockfile,
  native version manifest, native notice, and project license.
- Native executables and their runtime libraries are not currently bundled in
  the Go binary. A release that bundles them must include the notices from
  `native/NOTICE.md` and the corresponding upstream source/relinking terms.
- The project Apache-2.0 license does not relicense any codec dependency.
- gltfpack must be built with meshoptimizer compression (`-c`) as an opt-in
  EXT_meshopt path. Draco is not built, shipped, or emitted by Fabricum.

## References

- [libwebp cwebp options](https://developers.google.com/speed/webp/docs/cwebp)
- [libwebp license](https://github.com/webmproject/libwebp/blob/v1.6.0/COPYING)
- [libavif build and codec selection](https://github.com/AOMediaCodec/libavif/tree/v1.4.2)
- [libavif license](https://github.com/AOMediaCodec/libavif/blob/v1.4.2/LICENSE)
- [libaom license](https://aomedia.googlesource.com/aom/+/v3.14.1/LICENSE)
- [Basis Universal license](https://github.com/BinomialLLC/basis_universal/blob/v2_0_3/LICENSE)
- [Basis Universal notice](https://github.com/BinomialLLC/basis_universal/blob/v2_0_3/NOTICE)
- [Go license](https://go.dev/LICENSE)
- [Node license](https://github.com/nodejs/node/blob/v24.15.0/LICENSE)
