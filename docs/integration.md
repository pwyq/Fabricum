# Project integration

Use `sources` to restrict the editor to project-owned inputs:

```json
{
  "sourceSize": 1024,
  "sources": [
    {
      "path": "images/sample.png",
      "squareOutput": "delivery/sample-square.webp",
      "wideOutput": "delivery/sample-wide.webp"
    }
  ],
  "exportCommand": ["node", "register-export.mjs"]
}
```

## Sources

- The editor only shows configured sources.
- Source lists load at startup. Restart after changes.
- Each source may set its square and wide output paths.

## Export command

- Runs after output files are written.
- Receives a version 1 JSON receipt on stdin.
- Runs as an argument array without a shell.
- Uses the config directory as its working directory.
- Times out after 30 seconds.
- May run more than once; make it idempotent.
- A failure leaves the output files in place and fails the export response.
- Prefer an absolute executable path.
- Only use trusted config files. The command runs with your permissions.

The receipt contains:

- `processor` and absolute `source`.
- `request`: crops, format, quality, and lossless mode.
- `outputs`: role, path, dimensions, encoding, bytes, SHA-256, and crop.

Keep project naming, manifests, ownership, and alternate-format cleanup in the
export command. Fabricum does not remove alternate formats.

See [configuration](configuration.md) for path rules and CLI overrides.
