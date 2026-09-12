#!/usr/bin/env node

const fs = require("node:fs");

function extractReleaseNotes(changelogContent, tag) {
  const escapedTag = tag.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const headingPattern = new RegExp(
    `^## ${escapedTag}(?:\\s+-\\s+.+)?\\s*$`,
    "m",
  );
  const headingMatch = headingPattern.exec(changelogContent);

  if (!headingMatch) {
    throw new Error(`No changelog section found for ${tag}.`);
  }

  const sectionStart = headingMatch.index;
  const nextHeadingPattern = /^## v\d+\.\d+\.\d+(?:-[^\s]+)?(?:\s+-\s+.+)?\s*$/gm;
  nextHeadingPattern.lastIndex = sectionStart + headingMatch[0].length;
  const nextHeadingMatch = nextHeadingPattern.exec(changelogContent);
  const sectionEnd = nextHeadingMatch ? nextHeadingMatch.index : changelogContent.length;

  return changelogContent.slice(sectionStart, sectionEnd).trim();
}

function main() {
  const [tag, outputFlag, outputPath] = process.argv.slice(2);

  if (!tag) {
    console.error("Usage: node scripts/release/extract-release-notes.cjs v0.0.1 [--output path]");
    process.exit(1);
  }

  try {
    const notes = extractReleaseNotes(fs.readFileSync("CHANGELOG.md", "utf8"), tag);

    if (outputFlag === "--output") {
      if (!outputPath) {
        console.error("--output requires a path.");
        process.exit(1);
      }

      fs.writeFileSync(outputPath, `${notes}\n`);
      process.exit(0);
    }

    process.stdout.write(`${notes}\n`);
  } catch (error) {
    console.error(error.message);
    process.exit(1);
  }
}

if (require.main === module) {
  main();
}

module.exports = {
  extractReleaseNotes,
};
