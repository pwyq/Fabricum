# Fabricum

Fabricum is a game asset processing and optimization toolkit with a local
browser GUI for preparing 2D artwork.

## What it can do

- Choose a source and crop it visually into square and 4:3 artwork.
- Export PNG, WebP, or AVIF, with quality and lossless options.
- Preview encoded images and exact file sizes before exporting.

## Install

Requires Go 1.26.6 and Node.js 24.15 or newer. From the source checkout:

```text
npm ci
node scripts/build.mjs
```

The executable is written to `bin/fabricum.exe` on Windows or `bin/fabricum`
on Linux/macOS. Add `bin` to your PATH to use the `fabricum` command anywhere.

## Use

From the checkout on Windows:

```text
bin\fabricum.exe -source C:\images\sample.png -output-dir C:\images\delivery
```

On Linux/macOS:

```text
./bin/fabricum -source /images/sample.png -output-dir /images/delivery
```

Open the printed local URL to launch the GUI, adjust both crops, choose a
format, and export. The editor and processor stay on localhost.
Existing output files are replaced. When running from another directory, pass
`-encoder-directory` pointing to the checkout where you ran `npm ci`.
Use `-help` to list options.

See [documentation](docs/README.md) for configuration, project integration,
processing details, and development.

[Apache-2.0 license](LICENSE).
