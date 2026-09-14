# Changelog

## v0.2.2 - Indexed PNG output

2026-09-14

### Added

- Added opt-in, deterministic indexed PNG output with adaptive palettes of up to 256 colors.
- Added compatibility and regression coverage for palette encoding, transparency, quality, and invalid PNG modes.

### Changed

- Documented indexed PNG as a lossy, caller-approved optimization for size-sensitive image deliveries.

## v0.2.1 - Unspecified texture primaries

2026-09-14

### Fixed

- Accepted and reported unspecified KTX2 color primaries for non-color texture data.
- Covered linear normal and ARM textures in the standalone compatibility check.

## v0.2.0 - Standalone native release binaries

2026-09-13

### Added

- Added standalone Windows x64 and Linux x64 release binaries with embedded native tools and runtime libraries.
- Added machine-readable asset inspection, deterministic image transforms, loose KTX2 texture-set encoding, and static glTF optimization.
- Added self-describing processing receipts and a node-free compatibility check covering every supported asset workflow.
- Added embedded third-party notices and a content-addressed, verified native-tool cache.

### Changed

- Replaced Node and Sharp in the processing runtime with pinned native `cwebp`, `avifenc`, `basisu`, and `gltfpack` executables.
- Release downloads are now single executables that require no source checkout, Node.js installation, npm packages, or sidecar codec directory.

### Fixed

- Corrected transformed glTF bounds, external loose-glTF buffer inspection, KTX2 descriptor parsing, mirrored model winding, and extended WebP dimensions.

## v0.1.0 - Initial Fabricum release

2026-09-12

### Added

- Added a local browser GUI for selecting image sources and editing square and 4:3 crops.
- Added CLI mode for path-driven processing and export automation.
- Added PNG, WebP, and AVIF output with quality and lossless options.
- Added a standalone Go executable with embedded frontend assets and local-only processing.
