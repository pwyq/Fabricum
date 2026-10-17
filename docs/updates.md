# Release updates

`fabricum update` checks GitHub for the newest compatible stable Fabricum
release and installs it when needed. It runs without prompts or a browser and
writes one schema version 1 JSON result to stdout. The command supports the
published Windows x64 and Linux x64 executables.

```sh
fabricum update --min-version 0.2.4 --max-version 1.0.0
fabricum update --install-directory ./tools/fabricum --timeout 2m
```

## Compatibility and discovery

- `--min-version X.Y.Z` is inclusive; `--max-version X.Y.Z` is exclusive.
- Bounds accept stable versions without a `v` prefix. Either can be omitted.
  An empty or reversed range fails before contacting GitHub.
- Selection compares semantic versions numerically across published releases.
  Drafts, prereleases, and tags outside the repository's stable `vX.Y.Z`
  convention are excluded.
- A compatible running version newer than the selected release is kept. If
  the running version is outside explicit caller bounds, a compatible older
  release can be installed. No matching published release is a check failure.
- Every invocation checks GitHub, including reuse of an installed executable.
  HTTP failures, rate limits, malformed metadata, and timeouts fail the check;
  they never report that an unchecked installation is current.
- Discovery reads up to ten pages of 100 releases. Exceeding that limit fails
  explicitly instead of returning a possibly incomplete selection.

## Results and exit codes

A newly installed executable produces:

```json
{
  "schemaVersion": 1,
  "status": "updated",
  "executablePath": "/home/user/.cache/fabricum/releases/fabricum-0.2.5-linux-x64-<sha256>/fabricum-0.2.5-linux-x64",
  "version": "0.2.5",
  "updated": true
}
```

The path is absolute and points to the selected executable. Invoke that path
for subsequent processing; the updater does not launch the downloaded binary,
alter `PATH`, or overwrite its running executable.

- Exit `0`, `status: "updated"`, `updated: true`: this invocation published a
  new verified executable.
- Exit `0`, `status: "current"`, `updated: false`: the running executable is
  already compatible and current, or an identical verified installation was
  reused, including one installed by a concurrent process.
- Exit `1`, `status: "failed"`, `updated: false`: the check or installation
  failed. `error` explains the failure; `executablePath` and `version` are empty.
  A diagnostic is also printed to stderr. Existing installations remain intact.
- `--help` prints plain text and exits `0` without checking GitHub.

Consumers should check the exit code and status before using the selected path
and ignore unknown JSON properties for forward compatibility. A failed check
does not silently authorize fallback to an older executable; callers can choose
their own fallback policy using a previously saved path.

## Verification and installation

The default install root is `fabricum/releases` within the OS user cache
(`%LocalAppData%` on Windows, `$XDG_CACHE_HOME` or `~/.cache` on Linux).
`--install-directory` overrides this root; relative paths resolve from the
current directory. Use a directory owned by the current user and retain it for
as long as integrations use returned paths.

The updater downloads the platform executable and its exact `.sha256` sidecar
from the official `pwyq/Fabricum` GitHub release. Downloads require HTTPS;
redirects are limited to GitHub asset hosts. The checksum must name the selected
asset, and downloaded bytes must match its SHA-256. An empty executable, an
executable larger than 128 MiB, or an invalid checksum fails before publication.

Each attempt uses a private staging directory inside the install root. Only
after checksum verification, file flushing, and executable permissions succeed
does a same-filesystem directory rename publish the installation. Its directory
name includes the asset name and digest. Existing complete installs are hashed
again before reuse; corrupt or incomplete destinations are rejected.

This design works while the original Windows or Linux executable is running.
Concurrent processes can download independently; one publishes the destination
and the others verify and reuse it. There is no shared mutable launcher or lock
file. Previous versions are retained. A process killed during a download may
leave a `.update-*` staging directory; later attempts ignore it. Unused versions
and interrupted staging directories can be removed when no caller needs them.

`--timeout` bounds the entire check and installation (default `2m`, maximum
`10m`, greater than zero). Each HTTP request is additionally limited to 90
seconds, with bounded response headers, TLS handshakes, redirects, metadata
sizes, and binary size. The command makes no automatic retry after a failure.
