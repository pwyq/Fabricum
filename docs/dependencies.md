# Dependency review

## Direct dependencies

| Dependency | Purpose and scope | Necessary / built-in alternative | License and redistribution |
| --- | --- | --- | --- |
| Go 1.26.6 standard library | Runtime HTTP server, image decoding, bilinear processing, PNG, SHA-256, filesystem, subprocesses; development tests/vet/build | No third-party Go modules. Standard library covers these features. | BSD-3-Clause Go license; preserve the Go license when redistributing compiled executables. |
| Node.js >=24.15 | Runtime host for the optional WebP/AVIF encoder and export commands; development syntax/math tests and local scripts | Needed for Sharp. Go has no standard-library WebP/AVIF encoder. PNG server operation does not require Node. | Node distribution includes MIT and third-party notices. This repository does not redistribute Node. |
| Sharp 0.35.3 | Sole direct npm dependency, runtime WebP/AVIF encoding | Retained. Replacing it would introduce another native codec toolchain and risk output changes. | Apache-2.0 for Sharp; native dependency licenses differ. Installed by npm, not embedded in the Go binary. |

There are no development-only npm dependencies, framework dependencies, or internal
project packages. The private metadata-only package was replaced with an explicit
standalone manifest and lockfile. The encoder script was moved, not duplicated.
Sharp remains in the consuming project's asset tooling because other image
generators and audits also use it.

## Native packages and transitive dependencies

Sharp's native dependency is the largest component. The tested Windows package is
`@img/sharp-win32-x64` 0.35.3. Its installed README lists libvips, libheif, glib,
and several other libraries under LGPLv3 (via their later-version clauses), plus
BSD, MIT, MPL-2.0, font, image-library, and codec patent licenses. The package's
Apache label alone does not describe the whole native distribution. Other
platforms select different optional packages and must be reviewed separately.

The lockfile also includes small JavaScript helpers such as semver (ISC) and
`@img/colour` (MIT), and optional WASM/runtime packages. Keep the lockfile and
upstream notices with installed dependencies. Do not strip native package notices
or assume the project's Apache-2.0 license relicenses them.

The local release script emits only project source, the package lock, and the
project license. It excludes `node_modules`, native codecs, Node, and compiled Go
binaries. Before distributing a binary bundle or installers containing native
dependencies, review the exact platform notices and applicable source/relinking
requirements. Compatibility and patent rights are not inferred from npm metadata.

No unused direct dependency was found. No evidence of abandonment was found in the
reviewed Sharp documentation; maintenance status and vulnerability data can change.
The install reported one high-severity advisory, while the subsequent `npm audit
--json` returned zero advisories. Record a fresh audit for any distributed build;
this result does not prove absence of codec vulnerabilities.

Primary references:

- [Sharp installation and platform packages](https://sharp.pixelplumbing.com/install/)
- [Sharp license](https://github.com/lovell/sharp/blob/main/LICENSE)
- [Go license](https://go.dev/LICENSE)
- [Node license](https://github.com/nodejs/node/blob/main/LICENSE)
- [Renovate configuration](https://docs.renovatebot.com/configuration-options/)

The exact installed native package README and LICENSE remain authoritative for
its notices; dependency updates require another review. No game artwork, private
fixtures, repository history, credentials, or private infrastructure configuration
is included in the source release.
