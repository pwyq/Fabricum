#!/usr/bin/env node
const { execFileSync } = require('node:child_process')
const { isValidCommitSubject, validationError } = require('./validate-commit-message.cjs')

const [base, head] = process.argv.slice(2)
const sha = /^(?:[a-f0-9]{40}|[a-f0-9]{64})$/i
if (!sha.test(base ?? '') || !sha.test(head ?? '') || /^0+$/.test(base) || /^0+$/.test(head)) {
  console.error('Expected nonzero base and head commit SHAs.')
  process.exit(1)
}
try {
  const git = args => execFileSync('git', args, { encoding: 'utf8' }).trim()
  const commits = git(['rev-list', '--no-merges', `${base}..${head}`]).split(/\r?\n/).filter(Boolean)
  let failed = false
  for (const commit of commits) {
    const subject = git(['log', '-1', '--format=%s', commit])
    if (!isValidCommitSubject(subject)) {
      console.error(`${validationError()}\nInvalid commit: ${commit}\nSubject: ${subject}`)
      failed = true
    }
  }
  process.exitCode = failed ? 1 : 0
} catch (error) {
  console.error(`Cannot validate commit range: ${error.message}`)
  process.exitCode = 1
}
