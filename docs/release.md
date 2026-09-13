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

Create a source archive without publishing:

> npm run release:archive

## Gates

- Validate the tag, changelog, and release scripts.
- Run formatting, syntax, vet, tests, file-size checks, builds, and Linux races.
- Test on Ubuntu and Windows.
- Revalidate the tag and extract matching changelog notes.
- Create the annotated tag and GitHub Release.
- Create no tag when a gate fails.
- Accept a retry only when an existing tag targets the same commit.
