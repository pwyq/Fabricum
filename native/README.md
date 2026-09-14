# Native codec tools

WebP and AVIF encoding uses `cwebp` and `avifenc`; loose KTX2 encoding uses
the pinned `basisu` executable. Model optimization uses the statically linked
`gltfpack` executable built from the pinned meshoptimizer source. Put the
executables in `PATH`, or point Fabricum's `encoderDirectory` setting (or
`--encoder-directory`) at the directory containing them. Release executables
embed these four tools and extract them into a content-addressed user cache.

The pinned inputs are in [`versions.json`](versions.json). Basis Universal
2.0.3 is built from its integrity-checked source archive with CMake. Its
bundled Zstandard implementation is used for UASTC-Zstandard KTX2 output.
The selected AVIF encoder is libaom 3.14.1, selected explicitly with
`avifenc --codec aom`.

The full gltfpack build links meshoptimizer 1.2, Basis Universal 2.0.3, and
libwebp 1.6.0 statically. It therefore needs no Node package or separate
gltfpack installation at runtime. The build enables `-tc` Basis Universal
KTX2 textures, `-tw` WebP textures, and `-c` EXT_meshopt_compression.

## Linux

Use the official libwebp 1.6.0 Linux x64 archive for `cwebp` and the official
libavif 1.4.2 `linux-artifacts.zip` for `avifenc` and `avifdec`. The artifact
is built with the pinned local libaom 3.14.1 encoder. The installer builds
`basisu` 2.0.3 and the full gltfpack 1.2 tool from source and requires CMake.
Verify the digests in `versions.json` before putting the executables in
`PATH`.

## Windows

Use the official libwebp 1.6.0 x64 archive and the libavif 1.4.2
`windows-artifacts.zip`, verifying the digests in `versions.json`. The libavif
artifact is built with libaom; Fabricum still passes `--codec aom` so a
different codec cannot be selected accidentally. The installer builds
`basisu` 2.0.3 and gltfpack 1.2 from source with the installed CMake toolchain.
The self-extracting release design intentionally avoids static CGO bindings.
Decoder-only `dwebp` and `avifdec` tools remain development test dependencies
and are not embedded in releases.
