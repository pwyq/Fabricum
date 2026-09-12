#!/usr/bin/env node

const fs = require("node:fs");

const indicators = [
  "feat",
  "fix",
  "docs",
  "style",
  "refactor",
  "perf",
  "test",
  "build",
  "ci",
  "chore",
  "revert",
];
const pattern = new RegExp(
  `^(${indicators.join("|")}): .+ \\(#[1-9]\\d*\\)$`,
);

function getSubjectFromFile(messageFile) {
  return fs
    .readFileSync(messageFile, "utf8")
    .split(/\r?\n/)
    .map((line) => line.trim())
    .find((line) => line && !line.startsWith("#"));
}

function isValidCommitSubject(subject) {
  return Boolean(subject && pattern.test(subject));
}

function validationError() {
  return `Invalid commit message format.

Expected:
  <indicator>: <actual message> (#<GitHub Issue number>)

Allowed indicators:
  ${indicators.join(", ")}

Example:
  feat: add image export (#123)`;
}

function main() {
  const messageFile = process.argv[2];

  if (!messageFile || !fs.existsSync(messageFile)) {
    console.error("Commit message file not found.");
    process.exit(1);
  }

  if (isValidCommitSubject(getSubjectFromFile(messageFile))) {
    process.exit(0);
  }

  console.error(validationError());
  process.exit(1);
}

if (require.main === module) {
  main();
}

module.exports = {
  main,
  indicators,
  isValidCommitSubject,
  validationError,
};
