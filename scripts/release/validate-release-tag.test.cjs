const assert = require("node:assert/strict");
const test = require("node:test");

const {
  compareReleaseTags,
  isValidReleaseTag,
  validateRelease,
} = require("./validate-release-tag.cjs");

test("accepts strict release and project prerelease tags", () => {
  assert.equal(isValidReleaseTag("v1.2.3"), true);
  assert.equal(isValidReleaseTag("v0.1.0-alpha.1"), true);
  assert.equal(isValidReleaseTag("v0.1.0-beta.2"), true);
  assert.equal(isValidReleaseTag("v1.0.0-rc.1"), true);
});

test("rejects malformed or non-project release tags", () => {
  assert.equal(isValidReleaseTag("1.2.3"), false);
  assert.equal(isValidReleaseTag("v0.1"), false);
  assert.equal(isValidReleaseTag("foundation-release"), false);
  assert.equal(isValidReleaseTag("v0.1.0-foundation"), false);
  assert.equal(isValidReleaseTag("v01.2.3"), false);
});

test("compares final releases and prereleases", () => {
  assert.equal(compareReleaseTags("v0.0.2", "v0.0.1") > 0, true);
  assert.equal(compareReleaseTags("v0.1.0", "v0.0.9") > 0, true);
  assert.equal(compareReleaseTags("v0.1.0-alpha.2", "v0.1.0-alpha.1") > 0, true);
  assert.equal(compareReleaseTags("v0.1.0-beta.1", "v0.1.0-alpha.9") > 0, true);
  assert.equal(compareReleaseTags("v0.1.0", "v0.1.0-rc.1") > 0, true);
  assert.equal(compareReleaseTags("v0.1.0-alpha.1", "v0.1.0") < 0, true);
});

test("validates coherent version, changelog, and incremental tag", () => {
  const result = validateRelease({
    tag: "v0.0.2",
    versionContent: "0.0.2\n",
    changelogContent: "# Changelog\n\n## v0.0.2 - Patch\n\n- Fix\n",
    existingTags: ["v0.0.1"],
  });

  assert.deepEqual(result.errors, []);
});

test("rejects version mismatch and missing changelog section", () => {
  const result = validateRelease({
    tag: "v0.0.2",
    versionContent: "0.0.1\n",
    changelogContent: "# Changelog\n\n## v0.0.1 - Foundation\n",
    existingTags: ["v0.0.1"],
  });

  assert.match(result.errors.join("\n"), /VERSION must contain 0\.0\.2/);
  assert.match(result.errors.join("\n"), /CHANGELOG\.md must contain/);
});

test("rejects duplicate tag during preflight", () => {
  const result = validateRelease({
    tag: "v0.0.1",
    versionContent: "0.0.1\n",
    changelogContent: "## v0.0.1 - Foundation\n",
    existingTags: ["v0.0.1"],
    mode: "preflight",
  });

  assert.match(result.errors.join("\n"), /already exists locally/);
});

test("rejects non-incremental candidate tags", () => {
  const result = validateRelease({
    tag: "v0.0.1",
    versionContent: "0.0.1\n",
    changelogContent: "## v0.0.1 - Foundation\n",
    existingTags: ["v0.0.2"],
  });

  assert.match(result.errors.join("\n"), /greater than latest existing release tag/);
});

test("ignores the candidate tag during ci comparison", () => {
  const result = validateRelease({
    tag: "v0.0.2",
    versionContent: "0.0.2\n",
    changelogContent: "## v0.0.2 - Patch\n",
    existingTags: ["v0.0.1", "v0.0.2"],
    mode: "ci",
  });

  assert.deepEqual(result.errors, []);
});
