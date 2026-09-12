#!/usr/bin/env node

const {
  isValidCommitSubject,
  validationError,
} = require("./validate-commit-message.cjs");

function getPrTitle() {
  return process.argv[2] || process.env.PR_TITLE || "";
}

function main() {
  const title = getPrTitle().trim();

  if (isValidCommitSubject(title)) {
    process.exit(0);
  }

  console.error(validationError().replace("commit message", "PR title"));
  console.error(`Title: ${title || "(empty)"}`);
  process.exit(1);
}

if (require.main === module) {
  main();
}

module.exports = {
  main,
};
