#!/usr/bin/env node

const fs = require("node:fs");
const { execFileSync } = require("node:child_process");

const MAIN_BRANCH = "main";
const MAIN_REF = "refs/heads/main";
const ZERO_SHA = /^0+$/;

function isMainBranch(branch) {
  return branch === MAIN_BRANCH;
}

function isProtectedPush(push) {
  return push.localRef === MAIN_REF || push.remoteRef === MAIN_REF;
}

function parsePushInput(input) {
  return input
    .trim()
    .split(/\r?\n/)
    .filter(Boolean)
    .map((line) => {
      const [localRef, localSha, remoteRef, remoteSha] = line.trim().split(/\s+/);
      if (!localRef || !localSha || !remoteRef || !remoteSha) {
        throw new Error(`Malformed pre-push input: ${line}`);
      }
      return { localRef, localSha, remoteRef, remoteSha };
    });
}

function isMergedPullRequest(pullRequest, repository) {
  return Boolean(
    pullRequest &&
      pullRequest.merged_at &&
      pullRequest.base?.ref === MAIN_BRANCH &&
      (!repository || pullRequest.base?.repo?.full_name === repository),
  );
}

function isZeroSha(sha) {
  return typeof sha === "string" && ZERO_SHA.test(sha);
}

function git(args) {
  return execFileSync("git", args, { encoding: "utf8" }).trim();
}

function pushedCommitShas(event) {
  if (event.deleted) return [];

  if (event.before && !isZeroSha(event.before)) {
    try {
      return git(["rev-list", `${event.before}..${event.after}`])
        .split(/\r?\n/)
        .filter(Boolean);
    } catch (error) {
      throw new Error(`cannot enumerate pushed commits: ${error.message}`);
    }
  }

  return (event.commits ?? []).map((commit) => commit.id).filter(Boolean);
}

function rejectMainCommit() {
  const branch = git(["branch", "--show-current"]);
  if (isMainBranch(branch)) {
    console.error(
      "Commit rejected: committing directly on main is disabled. Create a feature branch instead.",
    );
    return false;
  }
  return true;
}

function rejectMainPush() {
  let pushes;
  try {
    pushes = parsePushInput(fs.readFileSync(0, "utf8"));
  } catch (error) {
    console.error(`Push rejected: ${error.message}`);
    return false;
  }

  let failed = false;
  for (const push of pushes) {
    if (!isProtectedPush(push)) continue;
    console.error(
      "Push rejected: direct pushes to main are disabled. Push to a feature branch instead.",
    );
    failed = true;
  }
  return !failed;
}

async function githubRequest(path, token) {
  const apiUrl = (process.env.GITHUB_API_URL || "https://api.github.com").replace(/\/$/, "");
  const response = await fetch(`${apiUrl}${path}`, {
    headers: {
      Accept: "application/vnd.github+json",
      Authorization: `Bearer ${token}`,
      "X-GitHub-Api-Version": "2022-11-28",
    },
  });
  if (!response.ok) {
    throw new Error(`GitHub API returned ${response.status} ${response.statusText}`);
  }
  const body = await response.json();
  if (!Array.isArray(body)) throw new Error("GitHub API returned an unexpected response");
  return body;
}

async function rejectDirectMainPushInCi() {
  const eventName = process.env.GITHUB_EVENT_NAME;
  if (eventName === "pull_request") {
    console.log("Pull request event accepted; main changes must arrive through this review path.");
    return true;
  }

  const eventPath = process.env.GITHUB_EVENT_PATH;
  if (!eventPath || !fs.existsSync(eventPath)) {
    throw new Error("GITHUB_EVENT_PATH is required for the main branch policy check");
  }
  const event = JSON.parse(fs.readFileSync(eventPath, "utf8"));
  if (event.ref !== MAIN_REF) {
    console.log("Main branch policy is only enforced for refs/heads/main.");
    return true;
  }

  const repository = process.env.GITHUB_REPOSITORY || event.repository?.full_name;
  const token = process.env.GITHUB_TOKEN;
  if (!repository || !token) {
    throw new Error("GITHUB_REPOSITORY and GITHUB_TOKEN are required for the main branch policy check");
  }

  const commits = pushedCommitShas(event);
  if (commits.length === 0) {
    console.log("No commits were pushed to main.");
    return true;
  }

  const results = await Promise.all(
    commits.map(async (sha) => {
      try {
        const pullRequests = await githubRequest(
          `/repos/${repository}/commits/${sha}/pulls?per_page=100`,
          token,
        );
        return { sha, pullRequests };
      } catch (error) {
        return { sha, error };
      }
    }),
  );

  let failed = false;
  for (const result of results) {
    if (result.error) {
      console.error(`Main policy could not verify ${result.sha}: ${result.error.message}`);
      failed = true;
      continue;
    }
    if (!result.pullRequests.some((pullRequest) => isMergedPullRequest(pullRequest, repository))) {
      console.error(
        `Main policy rejected ${result.sha}: every commit pushed to main must come from a merged pull request.`,
      );
      failed = true;
    }
  }

  return !failed;
}

async function main() {
  const command = process.argv[2];
  if (command === "commit") return rejectMainCommit() ? 0 : 1;
  if (command === "push") return rejectMainPush() ? 0 : 1;
  if (command === "ci") return (await rejectDirectMainPushInCi()) ? 0 : 1;

  console.error("Usage: protect-main.cjs <commit|push|ci>");
  return 1;
}

if (require.main === module) {
  main()
    .then((status) => {
      process.exitCode = status;
    })
    .catch((error) => {
      console.error(`Main branch policy failed: ${error.message}`);
      process.exitCode = 1;
    });
}

module.exports = {
  MAIN_BRANCH,
  MAIN_REF,
  isMainBranch,
  isMergedPullRequest,
  isProtectedPush,
  parsePushInput,
  pushedCommitShas,
};
