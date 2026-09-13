# Dependencies

## Runtime

| Dependency | Purpose | License notes |
| --- | --- | --- |
| Go 1.26.6 | Server, image processing, PNG, hashing, files, and tests. No third-party Go modules. | BSD-3-Clause. Preserve the Go license when redistributing binaries. |
| Node.js ≥24.15 | Sharp host, export commands, tests, and scripts. PNG processing does not need Node. | Includes MIT and third-party notices. Not redistributed here. |
| Sharp 0.35.3 | WebP and AVIF encoding. The only direct npm dependency. | Apache-2.0; bundled native libraries use other licenses. |

## Native packages

- Sharp selects platform-specific native packages.
- Native packages include libvips, libheif, glib, and other codec libraries.
- Licenses include LGPL, BSD, MIT, MPL-2.0, font, and image-library terms.
- Codec patent rights cannot be inferred from npm metadata.
- Review the exact package notices for every distributed platform.
- Keep the lockfile and upstream notices with installed dependencies.
- The project Apache-2.0 license does not relicense dependencies.

## Distribution

- Source archives include project source, lockfile, and project license.
- Source archives exclude `node_modules`, codecs, Node, and compiled Go binaries.
- Before bundling binaries or native packages, review notices and source/relinking terms.
- Run a fresh `npm audit` for every distributed build.

## References

- [Sharp installation and platform packages](https://sharp.pixelplumbing.com/install/)
- [Sharp license](https://github.com/lovell/sharp/blob/main/LICENSE)
- [Go license](https://go.dev/LICENSE)
- [Node license](https://github.com/nodejs/node/blob/main/LICENSE)
- [Renovate configuration](https://docs.renovatebot.com/configuration-options/)

Installed package README and LICENSE files are authoritative. Review them again
after dependency updates.
