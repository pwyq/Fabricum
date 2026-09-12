# Release Process

Fabricum releases are human-versioned and CI-authorized.

## Policy

- Root `VERSION` is the release source of truth and stores the version without
  `v`.
- `package.json` and `back-end/config.go` retain the same version for tooling
  and runtime receipts; `node scripts/check.mjs` verifies they do not drift.
- Git tags use the `VERSION` value with a `v` prefix.
- Tags must match one of these forms:
  - `vMAJOR.MINOR.PATCH`
  - `vMAJOR.MINOR.PATCH-alpha.N`
  - `vMAJOR.MINOR.PATCH-beta.N`
  - `vMAJOR.MINOR.PATCH-rc.N`
- Pushed release tags are immutable historical records. A bad release is fixed
  by publishing a newer release.
- GitHub creates the release page only after release validation and the full
  build gates pass.
- GitHub marks tags with a SemVer prerelease suffix as prereleases.

## Local Release

1. Update `VERSION`, `package.json`, and `back-end/config.go` to the same
   version.
2. Add a matching section to `CHANGELOG.md`.
3. Commit and push the release changes.
4. Run:

```bash
npm run release -- v0.1.0
```

Requirements:

- GitHub CLI installed and authenticated with `gh auth login`.
- Clean working tree.
- Current branch tracks `origin/<current-branch>` and is synchronized with it.
- Release commit is contained in `origin/main`.

The script validates release metadata, verifies the branch state, and dispatches
the GitHub Release workflow for the current branch. It does not create the tag
locally or publish directly. Monitor progress with `gh run watch`.

To create a source archive without publishing a release, run:

```bash
npm run release:archive
```

## Release Gates

CI first validates the tag, changelog, and release scripts. It then runs the
same formatting, JavaScript syntax, Go vet, tests, file-size checks, build, and
Linux race checks used by the normal build workflow on Ubuntu and Windows.

Only after all gates pass does CI revalidate the tag, extract the matching
`CHANGELOG.md` section, create and push the annotated tag, and create the GitHub
Release. Failed gates create no tag. Finalization retries accept an existing
tag only when it targets the same commit.
