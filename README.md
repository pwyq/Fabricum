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
bash install.sh
```

The installer installs the pinned encoder dependencies, downloads Go modules,
builds the standalone executable, and configures the repository's local Git
hooks. The hooks keep work on feature branches and reject direct pushes to
`main`.

The executable is written to `bin/fabricum.exe` on Windows or `bin/fabricum`
on Linux/macOS. Add `bin` to your PATH to use the `fabricum` command anywhere.

## Use

Running the executable without arguments opens the GUI and lets you choose a
PNG, JPEG, or GIF source in the browser:

```text
bin\fabricum.exe
```

Use `-mode gui` to make that choice explicit. For path-driven use, select CLI
mode and provide the source path:

```text
bin\fabricum.exe -mode cli -source C:\images\sample.png -output-dir C:\images\delivery
```

On Linux/macOS, the equivalent commands are:

```text
./bin/fabricum
./bin/fabricum -mode cli -source /images/sample.png -output-dir /images/delivery
```

GUI mode opens the local URL automatically; CLI mode prints it without opening
a browser. In either mode, adjust both crops, choose a format, and export in
the local editor. The editor and processor stay on localhost.
Existing output files are replaced. When running from another directory, pass
`-encoder-directory` pointing to the checkout where you ran `bash install.sh`.
The installed executable finds that checkout automatically when it remains next
to its `bin` directory.
Use `-help` to list options.

See [documentation](docs/README.md) for configuration, project integration,
processing details, and development.

[Apache-2.0 license](LICENSE).
