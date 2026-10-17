# Offline SVG optimization

Run SVG-to-SVG optimization in place:

```text
fabricum optimize-svg icon.svg artwork.svg
fabricum optimize-svg -- -icon.svg
```

Flags precede file paths; `--` permits paths starting with a dash. The command
accepts individual regular files, including paths with spaces. It writes a
versioned JSON size report to stdout. Every file receives a result, including
failures; any failure produces a nonzero exit status after the report is written.
Successful files in a mixed batch commit independently.

The optimizer is **fabricum-svg 1.0.0**, compiled into Fabricum using Go's
standard library. Releases work offline without Node, npm, a browser, an SVG
editor, or another executable. The public Go API exposes `OptimizeSVG`,
`OptimizeSVGFiles`, `OptimizeSVGArgs`, and their request/result/receipt types.

## Cleanup and preservation

The supported profile is static, namespaced SVG rendered as a standalone image
by SVG2 browsers (Chromium/Edge). Representative icons, gradients, clipping,
masks, filters, prefixed XML, and text/CSS fixtures are compared pixel for pixel
in a headless browser during tests. A browser is only a development dependency.

Cleanup removes XML declarations, comments, redundant markup spacing, empty
closing tags, and insignificant whitespace in element-only containers. For
documents without style attributes, style elements, or classes, it also:

- removes SVG metadata and known Inkscape/Sodipodi/Adobe editor extensions without IDs;
- unwraps attribute-free drawing groups and removes exact identity group transforms;
- removes selected default gradient, clipping, mask, pattern, and filter attributes
  when they do not override an `href` template;
- shares identical gradient stop definitions through SVG2 `href` templates,
  keeping both original gradient IDs.

`viewBox`, dimensions, IDs, references, accessibility labels, geometry, number
precision, font declarations, and significant text spacing are retained.
Templates, masks, filters, and definitions with descendant IDs are preserved.
There is no path simplification or precision-reduction option in this profile.
Text rendering continues to depend on fonts available to the consuming renderer.

Styles and classes disable structural cleanup and whitespace removal throughout
the document, with an explicit diagnostic in the result. CSS text, attribute
values, namespace prefixes, entity spelling, CDATA, and text remain verbatim;
markup between attributes can still shrink. This protects ancestor, attribute,
and child selectors. External CSS supplied later by a consuming application
cannot be inferred: optimize assets as standalone images, or retain a class/style
on the document to request structure preservation.

## Secure offline profile

The optimizer parses XML without executing code, loading a DTD, fetching a URL,
reading linked files, or invoking a renderer. Namespace declarations are names,
not resource downloads. It rejects the following with a file-specific error:

- DTDs/entity declarations, processing instructions (except the initial XML declaration),
  scripts, event attributes, `foreignObject`, animation, and unknown rendering elements;
- external, relative, file, and data references; `href`, `xlink:href`, CSS `url()`
  paints, and accessibility references must resolve to IDs in the same document;
- `xml:base`, external fonts/stylesheets, CSS at-rules, CSS escapes/comments, and
  unsupported CSS functions that could conceal or generate external resources;
- malformed XML, duplicate attributes or IDs, unbound prefixes, and dangling references.

Common color and arithmetic functions, CSS variables, and SVG transforms are
accepted; unsupported constructs fail explicitly rather than being rewritten.
ID names use the conservative ASCII form `[A-Za-z_][A-Za-z0-9_.:-]*`.
SVGZ, bitmap tracing, raster export, embedded images, and interactive documents
are outside this command's profile. Inputs are limited to 16 MiB, 200,000 XML
tokens, 128 element levels, 256 attributes per element, and 256 in-scope namespaces.

## File safety and report

The candidate is reparsed and all references are revalidated. Fabricum replaces
the source only when the candidate is strictly smaller. Equal/larger results
leave both original bytes and timestamps alone. A synchronized temporary file
in the same directory is committed atomically, with original permissions and
checks for concurrent source changes. Parse, policy, cancellation, and write
failures leave the original intact; symbolic-link inputs are rejected.

```json
{
  "schemaVersion": 1,
  "processor": "fabricum/0.2.4",
  "files": [{
    "path": "/absolute/path/icon.svg",
    "inputBytes": 1000,
    "outputBytes": 700,
    "savingsBytes": 300,
    "savingsPercent": 30,
    "format": "svg",
    "optimizer": "fabricum-svg",
    "optimizerVersion": "1.0.0",
    "changed": true
  }]
}
```

Optional `diagnostics` explains conservative preservation. Failed results include
`error`, `changed: false`, and zero savings. Size fields describe the retained
file when it could be read. A receipt/output-stream error is reported after any
already successful file replacements, which are not rolled back.

## Engine decision and licenses

[SVGO](https://github.com/svg/svgo) offers extensive optimization and uses the
MIT license, but its CLI requires Node and its browser bundle would require an
additional embedded JavaScript engine and dependency inventory.
[svgcleaner](https://github.com/RazrFalcon/svgcleaner) offers a standalone Rust
executable under GPL-2.0, but its upstream repository is archived and documents
limited CSS handling, including extracting style rules and resolving `inherit`
and `currentColor`. Neither engine is shipped by Fabricum.

The narrower in-tree engine prioritizes exact text/style preservation and the
existing standalone distribution. Version `1.0.0` identifies the source-defined
profile independently of the Fabricum release version. Its code is covered by
Fabricum's Apache-2.0 license. Its only runtime dependency is the bundled Go
standard library/runtime (Go license, BSD-3-Clause); there are no additional SVG
packages, native tools, or transitive dependencies. The version/license inventory
is embedded in `fabricum third-party-notices` and the engine version appears in
every size result. Review this profile and its rendering fixtures when changing
the optimizer version.
