# Releases

## Policy

- Releases are human-versioned and CI-authorized.
- `VERSION` is authoritative and omits the `v` prefix.
- `package.json` and `back-end/internal/editor/config.go` must match `VERSION`.
- `node scripts/check.mjs` checks version consistency.
- Tags are immutable. Fix a bad release with a newer version.
- GitHub publishes only after all release gates pass.
- SemVer suffixes create GitHub prereleases.

Allowed tags:

- `vMAJOR.MINOR.PATCH`
- `vMAJOR.MINOR.PATCH-alpha.N`
- `vMAJOR.MINOR.PATCH-beta.N`
- `vMAJOR.MINOR.PATCH-rc.N`

## Publish

1. Update `VERSION`, `package.json`, and `back-end/internal/editor/config.go` to the same
   version.
2. Add a matching section to `CHANGELOG.md`.
3. Commit and push the release changes.
4. Run:

```bash
npm run release -- v0.1.0
```

Requirements:

- Authenticated GitHub CLI: `gh auth login`.
- Clean working tree.
- Current branch tracks and matches its remote branch.
- Release commit exists on `origin/main`.

The command validates metadata and branch state, then dispatches the release
workflow. It does not tag or publish locally.

Monitor CI:

> gh run watch

Create a source archive for local inspection without publishing it:

> npm run release:archive

After building the executable and installing the pinned native tools, create
and clean-check a standalone binary for the current platform with:

> npm run release:binary

The command writes the platform binary and its SHA-256 file to `bin`.
It does not publish or contact GitHub.

## Standalone binaries

The release workflow builds and tests one executable for each supported platform:

- `fabricum-<version>-linux-x64`
- `fabricum-<version>-windows-x64.exe`

Each executable contains compressed copies of `cwebp`, `avifenc`, `basisu`,
`gltfpack`, required native runtime libraries, the project license, and the
third-party notice, patent, and version inventory. Decoder-only tools are not
included. `fabricum third-party-notices` prints the embedded legal inventory.
The expected standalone binary size is 15–50 MiB.

Before upload, the workflow copies only the executable into a temporary
directory and runs `compatibility-check` with an empty `PATH`. This proves
inspection, PNG/WebP/AVIF output, loose KTX2 encoding, and glTF optimization
without a source checkout, package installation, Node, Sharp, or a sidecar
codec directory.

At runtime, Fabricum verifies and extracts the embedded tools into a
content-addressed directory under the current user's cache. The release
download itself remains one executable per platform.

Only Linux x64 and Windows x64 are declared release platforms. macOS source
builds are not published until equivalent standalone-binary and compatibility
checks exist.

## Gates

- Validate the tag, changelog, and release scripts.
- Run formatting, syntax, vet, tests, file-size checks, builds, and Linux races.
- Test on Ubuntu and Windows.
- Revalidate the tag and extract matching changelog notes.
- Build, clean-check, and upload each standalone executable and its SHA-256 file.
- Create the annotated tag and GitHub Release with both platform executables.
- Create no tag when a gate fails.
- Accept a retry only when an existing tag targets the same commit.
