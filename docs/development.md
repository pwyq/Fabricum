# Development

## Repository layout

`front-end/static` owns the editor, and `front-end/tests` contains crop math
tests. `front-end/assets.go` embeds those assets directly for the Go server.
`back-end` owns processing, HTTP, configuration, encoding, and Go tests;
`back-end/cmd/fabricum` is the executable entry point.

The root keeps shared module/package metadata and repository tooling. One Go
module allows the backend to import embedded frontend assets without a generated
copy or build-time synchronization step. No application Go files live at root.

## CI guards

`.github/workflows/build.yml` runs the same local check and build commands on
Ubuntu and Windows for every PR to main, pushes to main, and manual runs. Linux
also runs Go race tests. Checks include Go formatting/vet/tests, JavaScript
syntax, crop math, commit guard tests, and compilation of the embedded CLI.

`.github/workflows/commit-message.yml` checks the PR title and each non-merge
commit introduced by the PR. Use `<type>: <message> (#<issue-number>)`, for example
`feat: add image export (#123)`. Types: feat, fix, docs, style, refactor, perf,
test, build, ci, chore, revert. The issue number must be positive; the guard checks
format without fetching issues. Renovate-authored PRs are exempt from this
message policy, but still run the build guard.

The workflows use read-only repository permissions and no deployment secrets.
Their files are prepared locally; no remote settings or required-status branch
rules are changed by these scripts.

## Checks and local release

Run commands from the repository root after `npm ci`. Go can download the required
toolchain when permitted. Sharp is pinned for WebP/AVIF; PNG processing needs only Go.

```text
node scripts/check.mjs
node scripts/build.mjs
```

`node scripts/check.mjs` is the portable CI entry point: Go formatting, vet,
tests, compilation, JS syntax, crop math tests, and version consistency. No UI or
browser tests are required. Run `npm audit` for the installed dependency graph.
No JavaScript build framework or TypeScript toolchain is needed.

`node scripts/release.mjs` verifies the checkout and creates a source archive and
SHA-256 file under `bin`. It does not publish, tag, commit, or contact GitHub.
Use semantic versions starting at `0.1.0`; a future release tag is `v0.1.0`.
Update both `back-end/config.go` and package metadata/lockfile when changing versions.
The Go module currently has the local name `fabricum`; choose a public module
identity before offering `go install ...@version`.

Renovate configuration is prepared but inactive until a maintainer installs a bot.
Updates are weekly, delayed 14 days, grouped for minor/patch releases, never
automerged, and majors require dashboard approval. Review codec output changes
with dependency updates. Workflow definitions are included; no release service
is configured.

See [dependency review](dependencies.md) for native codec licensing and redistribution.
