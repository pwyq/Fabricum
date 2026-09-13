# Fabricum

Fabricum is a game asset processing and optimization toolkit with a local
browser GUI for preparing 2D artwork.

## Functions

- Crop source image visually into square and 4:3 artwork.
- Run deterministic noninteractive 2D transforms, including channel packing.
- Export PNG, WebP, or AVIF, with quality and lossless options, reducing file sizes up to 95%
- Preview encoded images and exact file sizes before exporting.

## Install

Requires Go 1.26.6, Node.js 24.15 or newer for repository tooling, and the
native `cwebp` and `avifenc` tools for WebP/AVIF output. From the source checkout:

> bash install.sh

The executable is written to `bin/fabricum.exe` on Windows or `bin/fabricum`
on Linux/macOS. Add `bin` to your PATH to use the `fabricum` command anywhere.

## Use

GUI mode:

> bin\fabricum.exe

CLI mode (Windows):

> bin\fabricum.exe --source input-image-path-here --output output-directory-here

CLI mode (Unix):

> ./bin/fabricum --source input-image-path-here --output output-directory-here

Machine-readable asset inspection:

> bin\fabricum.exe inspect image.png model.glb texture.ktx2

Noninteractive image transforms:

> bin\fabricum.exe transform transform.json

Short flags are also available: `-s path -o path`. A bare launch opens the GUI;
supplying a source or output path automatically selects CLI mode.

See [documentation](docs/README.md) for configuration, project integration,
processing details, and development.

[Apache-2.0 license](LICENSE).
