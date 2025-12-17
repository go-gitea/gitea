import {guessPreviousReleaseTag} from './repo-release.ts';

test('guessPreviousReleaseTag', async () => {
  expect(guessPreviousReleaseTag('v0.9', ['v1.0', 'v1.2', 'v1.4', 'v1.6'])).toBe('');
  expect(guessPreviousReleaseTag('1.3', ['v1.0', 'v1.2', 'v1.4', 'v1.6'])).toBe('v1.2');
  expect(guessPreviousReleaseTag('rel/1.3', ['v1.0', 'v1.2', 'v1.4', 'v1.6'])).toBe('v1.2');
  expect(guessPreviousReleaseTag('v1.3', ['rel/1.0', 'rel/1.2', 'rel/1.4', 'rel/1.6'])).toBe('rel/1.2');
  expect(guessPreviousReleaseTag('v2.0', ['v1.0', 'v1.2', 'v1.4', 'v1.6'])).toBe('v1.6');
});
