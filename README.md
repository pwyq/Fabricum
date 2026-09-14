# Fabricum

Fabricum is a game asset processing and optimization toolkit with a local
browser GUI for preparing 2D artwork.

## Functions

- Crop source image visually into square and 4:3 artwork.
- Run deterministic noninteractive 2D transforms, including channel packing.
- Build loose ETC1S and UASTC-Zstandard KTX2 texture sets from ordinary images.
- Export PNG, WebP, or AVIF, with quality and lossless options, reducing file sizes up to 95%
- Preview encoded images and exact file sizes before exporting.

## Install

Prebuilt standalone executables support Windows x64 and Linux x64. Each
executable contains its required native tools, so running a release does not
require Node.js, Go, CMake, an npm install, or a separate codecs directory.

Source development requires Go 1.26.6, Node.js 24.15 or newer, and CMake.
Set up the development environment from the source checkout:

> bash install.sh

Compile the final standalone executable for the current platform:

> npm run release:binary

The result is written to `bin/fabricum-<version>-windows-x64.exe` on Windows or
`bin/fabricum-<version>-linux-x64` on Linux. This is the complete release
artifact. For a quicker development build that uses the locally installed
native tools, run `npm run build`; its unversioned output is removed by the
next release build.

macOS is not a supported release platform until its native binary and clean
compatibility checks are available.

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

Loose KTX2 texture sets:

> bin\fabricum.exe texture-set texture-set.json

Short flags are also available: `-s path -o path`. A bare launch opens the GUI;
supplying a source or output path automatically selects CLI mode.

See [documentation](docs/README.md) for configuration, project integration,
processing details, the compatibility check, and development.

[Apache-2.0 license](LICENSE).
