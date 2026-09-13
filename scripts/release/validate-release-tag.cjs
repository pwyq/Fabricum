#!/usr/bin/env node

const fs = require("node:fs");
const { execFileSync } = require("node:child_process");

const releaseTagPattern =
  /^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-(alpha|beta|rc)\.(0|[1-9]\d*))?$/;
const prereleaseRank = new Map([
  ["alpha", 0],
  ["beta", 1],
  ["rc", 2],
]);

function git(args, options = {}) {
  try {
    return execFileSync("git", args, {
      encoding: "utf8",
      stdio: ["ignore", "pipe", options.allowFailure ? "ignore" : "pipe"],
    }).trim();
  } catch (error) {
    if (options.allowFailure) {
      return "";
    }

    throw error;
  }
}

function parseReleaseTag(tag) {
  const match = releaseTagPattern.exec(tag);

  if (!match) {
    return null;
  }

  return {
    major: Number(match[1]),
    minor: Number(match[2]),
    patch: Number(match[3]),
    prerelease: match[4]
      ? {
          label: match[4],
          number: Number(match[5]),
        }
      : null,
  };
}

function isValidReleaseTag(tag) {
  return Boolean(parseReleaseTag(tag));
}

function compareNumbers(left, right) {
  if (left === right) {
    return 0;
  }

  return left > right ? 1 : -1;
}

function comparePrerelease(left, right) {
  if (!left && !right) {
    return 0;
  }

  if (!left) {
    return 1;
  }

  if (!right) {
    return -1;
  }

  const labelComparison = compareNumbers(
    prereleaseRank.get(left.label),
    prereleaseRank.get(right.label),
  );

  if (labelComparison !== 0) {
    return labelComparison;
  }

  return compareNumbers(left.number, right.number);
}

function compareReleaseTags(leftTag, rightTag) {
  const left = parseReleaseTag(leftTag);
  const right = parseReleaseTag(rightTag);

  if (!left || !right) {
    throw new Error("Both tags must match the release tag policy.");
  }

  for (const key of ["major", "minor", "patch"]) {
    const comparison = compareNumbers(left[key], right[key]);

    if (comparison !== 0) {
      return comparison;
    }
  }

  return comparePrerelease(left.prerelease, right.prerelease);
}

function tagToVersion(tag) {
  return tag.startsWith("v") ? tag.slice(1) : tag;
}

function changelogHasTag(changelogContent, tag) {
  const escapedTag = tag.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  return new RegExp(`^## ${escapedTag}(?:\\s+-\\s+.+)?\\s*$`, "m").test(
    changelogContent,
  );
}

function getLatestExistingReleaseTag(existingTags, candidateTag, mode) {
  const comparableTags = existingTags
    .filter(isValidReleaseTag)
    .filter((tag) => mode !== "ci" || tag !== candidateTag);

  return comparableTags.sort(compareReleaseTags).at(-1) ?? null;
}

function validateRelease({
  tag,
  versionContent,
  changelogContent,
  existingTags = [],
  remoteTagExists = false,
  mode = "preflight",
}) {
  const errors = [];

  if (!tag) {
    errors.push("Release tag is required.");
    return { errors };
  }

  if (!isValidReleaseTag(tag)) {
    errors.push(
      "Release tag must match vMAJOR.MINOR.PATCH or vMAJOR.MINOR.PATCH-(alpha|beta|rc).N.",
    );
    return { errors };
  }

  const version = tagToVersion(tag);
  const versionFileValue = versionContent.trim();

  if (versionFileValue !== version) {
    errors.push(`VERSION must contain ${version} to match ${tag}.`);
  }

  if (!changelogHasTag(changelogContent, tag)) {
    errors.push(`CHANGELOG.md must contain a section heading for ${tag}.`);
  }

  if (mode === "preflight" && existingTags.includes(tag)) {
    errors.push(`Release tag ${tag} already exists locally.`);
  }

  if (mode === "preflight" && remoteTagExists) {
    errors.push(`Release tag ${tag} already exists on origin.`);
  }

  const latestTag = getLatestExistingReleaseTag(existingTags, tag, mode);

  if (latestTag && compareReleaseTags(tag, latestTag) <= 0) {
    errors.push(
      `Release tag ${tag} must be greater than latest existing release tag ${latestTag}.`,
    );
  }

  return { errors };
}

function getExistingTags() {
  return git(["tag", "--list", "v*"])
    .split(/\r?\n/)
    .map((tag) => tag.trim())
    .filter(Boolean);
}

function getRemoteTagExists(tag) {
  return Boolean(git(["ls-remote", "--tags", "origin", `refs/tags/${tag}`]));
}

function parseArgs(args) {
  const modeIndex = args.indexOf("--mode");
  const mode = modeIndex === -1 ? "preflight" : args[modeIndex + 1];
  const tag = args.find(
    (arg, index) => index !== modeIndex + 1 && arg !== "--mode",
  );

  return { tag, mode };
}

function main() {
  const { tag, mode } = parseArgs(process.argv.slice(2));

  if (!["preflight", "ci"].includes(mode)) {
    console.error("Mode must be preflight or ci.");
    process.exit(1);
  }

  const result = validateRelease({
    tag,
    versionContent: fs.readFileSync("VERSION", "utf8"),
    changelogContent: fs.readFileSync("CHANGELOG.md", "utf8"),
    existingTags: getExistingTags(),
    remoteTagExists:
      mode === "preflight" && isValidReleaseTag(tag)
        ? getRemoteTagExists(tag)
        : false,
    mode,
  });

  if (result.errors.length === 0) {
    process.exit(0);
  }

  console.error(
    `Invalid release tag.\n\n${result.errors
      .map((error) => `- ${error}`)
      .join("\n")}`,
  );
  process.exit(1);
}

if (require.main === module) {
  main();
}

module.exports = {
  compareReleaseTags,
  isValidReleaseTag,
  parseReleaseTag,
  validateRelease,
};
