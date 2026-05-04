export default {
  extends: ['@commitlint/config-conventional'],
  rules: {
    // restrict to a curated set of types; intentionally exclude "chore"
    // because it is overused (especially by AI-assisted contributions) and
    // rarely conveys useful information about a change.
    'type-enum': [
      2,
      'always',
      ['build', 'ci', 'docs', 'feat', 'fix', 'perf', 'refactor', 'revert', 'style', 'test'],
    ],
  },
};
