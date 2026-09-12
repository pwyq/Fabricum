# Project integration

Projects can supply a `sources` array instead of an initial source:

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

The editor selects only from that list. Lists are loaded at startup; restart after
changing them. The optional `exportCommand` receives a JSON receipt on stdin after
the files have been written. It runs directly as an argument array, without a
shell, in the config file's directory with a 30-second timeout. Relative executable
paths containing a separator resolve from the process working directory; prefer
an absolute executable path. A nonzero exit fails the export response and reports
that outputs were already written. Repeated exports can repeat the command, so
registration should be idempotent. Only load configuration files you trust: an
export command executes local code with your permissions.

Receipt schema version 1 includes `processor`, absolute `source`, `request`
(square/wide crop rectangles, format, quality, lossless), and `outputs` (role,
path, dimensions, encoding options, bytes, SHA-256, crop). Project-specific naming,
manifest schemas, ownership, and removal of alternate formats belong in the
project's registration command. The CLI never removes alternate output formats.

See [configuration](configuration.md) for path resolution and CLI overrides.
