# Development

## Repository layout

- `front-end/static`: browser editor.
- `front-end/tests`: crop math tests.
- `front-end/assets.go`: embedded editor assets.
- `back-end/fabricum.go`: stable host interface used by the executable and integrations.
- `back-end/internal/cli`: flags, JSON launch configuration, and export-command integration.
- `back-end/internal/editor`: local HTTP editor, source handling, security, and GUI lifecycle.
- `back-end/internal/processing`: crop, resize, native encoding, hashing, and delivery file writes.
- `back-end/cmd/fabricum`: executable entry point.
- Root: Go/npm metadata and repository tooling.

The single Go module lets the backend import embedded frontend assets directly.

## Setup and checks

From the repository root:

> bash install.sh

> node scripts/check.mjs

> node scripts/build.mjs

- `install.sh`: install dependencies, configure hooks, and build.
- `check.mjs`: formatting, syntax, vet, tests, compilation, and version checks.
- `build.mjs`: build the embedded executable.
- `npm audit`: inspect the installed npm dependency graph.
- Linux CI also runs Go race tests.
- No browser runner, frontend framework, or TypeScript toolchain is required.
- WebP/AVIF behavior tests need the pinned `cwebp` and `avifenc` tools; see
  [`dependencies.md`](dependencies.md) and [`../native/README.md`](../native/README.md).

## Git rules

- Work on a branch; local hooks reject commits and pushes to `main`.
- CI requires each new `main` commit to come from a merged PR.
- Protect `main` with required PRs and the `Require pull request for main` check.
- Hooks are local guardrails; CI and branch protection remain authoritative.
- Workflows use read-only repository permissions and no deployment secrets.

Commit and PR titles use:

> `<type>: <message> (#<issue-number>)`

Example:

> `feat: add image export (#123)`

Allowed types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`,
`build`, `ci`, `chore`, `revert`.

## Versions and updates

- `VERSION` is authoritative.
- `package.json` and `back-end/internal/editor/config.go` must match it.
- `npm run release:archive` creates a source archive and SHA-256 file in `bin`.
- Archives do not publish, tag, commit, or contact GitHub.
- The Go module name is local; choose a public identity before supporting `go install`.
- Renovate is inactive until installed.
- Renovate waits 14 days, groups minor/patch updates, and never automerges.
- Review codec output changes after dependency updates.

See [releases](release.md) and [dependencies](dependencies.md).
