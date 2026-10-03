import {execFileSync} from 'node:child_process';

test('release-notes groups subjects by type and skips chores, ci and non-conventional commits', () => {
  const subjects = [
    'fix: correct totals (#1)',
    'feat(api)!: drop v1 endpoints (#2)',
    'chore(deps): update deps (#3)',
    'ci: tune lint (#4)',
    '[skip ci] Updated translations via Crowdin',
    'docs: describe widgets (#5)',
    'refactor: move code (#6)',
    'chore!: drop old node (#7)',
  ].join('\n');
  expect(execFileSync(process.execPath, ['tools/ci-tools.ts', 'release-notes'], {input: subjects, encoding: 'utf8'})).toBe(`# Breaking Changes

- feat(api)!: drop v1 endpoints (#2)
- chore!: drop old node (#7)

# Bug Fixes

- fix: correct totals (#1)

# Documentation

- docs: describe widgets (#5)

# Miscellaneous

- refactor: move code (#6)
`);
});
