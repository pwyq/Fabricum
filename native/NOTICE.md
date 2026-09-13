# Native codec notices

Fabricum invokes the following native command-line tools. They are not Go or
Node dependencies and are not embedded in the executable yet.

| Tool | Pinned source | Encoding dependency | License notice |
| --- | --- | --- | --- |
| `cwebp` | libwebp 1.6.0 | libwebp and libsharpyuv | [libwebp license](https://github.com/webmproject/libwebp/blob/v1.6.0/COPYING) |
| `avifenc` | libavif 1.4.2 | libaom 3.14.1 (`aom` only) | [libavif license](https://github.com/AOMediaCodec/libavif/blob/v1.4.2/LICENSE), [libaom license](https://aomedia.googlesource.com/aom/+/v3.14.1/LICENSE) |

The libavif build must enable only its `aom` encoder and must not silently
select rav1e, SVT-AV1, or another AV1 implementation. Keep the upstream
license and patent notices for every native artifact distributed with a
release. The exact URLs, versions, Windows artifact digest, and Linux build
inputs are recorded in [`versions.json`](versions.json).
