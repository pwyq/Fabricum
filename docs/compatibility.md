# Compatibility check

Build the executable, then run the node-free end-to-end check:

```text
go build -trimpath -o bin/fabricum ./back-end/cmd/fabricum
bin/fabricum compatibility-check > compatibility-receipt.json
```

On Windows, run `bin\fabricum.exe compatibility-check` instead. The native
`cwebp`, `avifenc`, `basisu`, and `gltfpack` tools must be beside the
executable in `codecs`, or available on `PATH`.

The check creates disposable, deterministic fixtures and removes them after a
successful run. It exercises:

- browserless preview and export for 512×512 and 768×576 PNG, WebP, and AVIF;
- contained transparent 256×256 and 512×512 PNG sprites;
- a complete 1024×1024 ETC1S base-color and UASTC-Zstandard normal/ARM set;
- ordered inspection of PNG, JPEG, GIF, WebP, AVIF, KTX2, glTF, GLB, and BIN;
- static-prop transforms, centering, mirrored winding repair, material
  deduplication, buffer compaction, and an EXT_meshopt_compression build.

The command writes one schema-versioned JSON receipt containing the nested
workflow receipts. Every generated output is checked against its recorded
byte count and SHA-256, and native processor identities are checked as well.
Inspection facts intentionally contain no content hashes. A nonzero exit means
a fixture assertion failed or a required native tool is unavailable.

The fixtures contain no project artwork, manifests, runtime groups, delivery
policy, or project naming conventions. The command uses only the Go
executable and bundled/native codec tools; it does not start a browser or
invoke Node or Sharp.
