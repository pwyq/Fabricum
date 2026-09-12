# Processing behavior

The editor defaults to WebP quality 90. PNG is lossless; WebP and AVIF offer
quality 1–100 and lossless mode. AVIF uses 4:4:4 chroma and maximum effort, which
can make previews slower. Superseded preview requests cancel the encoder process.
PNG output is deterministic for identical inputs/crops and the same toolchain.
For WebP/AVIF reproducibility, keep Sharp, its native codecs, and platform fixed.

Export replaces existing output files. Each file is synchronized to a temporary
file before replacement, with restoration attempted on failure. Multiple outputs
and external registration are not a transaction. Fix errors and retry export;
use one writer per output set or shared project manifest. Import replaces the
selected source with a validated PNG after the editor asks for confirmation.

The server binds only to loopback, rejects foreign Host/Origin headers, and requires
a random session token for mutations. It is an authoring tool for a trusted local
user, not a multi-user service. Do not expose it through a proxy or port tunnel.

3D processing and general-purpose asset pipelines are outside this release.
