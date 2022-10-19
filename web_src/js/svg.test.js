import {expect, test} from 'vitest';
import {svg} from './svg.js';

test('svg', () => {
  expect(svg('octicon-repo')).toMatch(/^<svg/);
  expect(svg('octicon-repo', 16)).toContain('width="16"');
  expect(svg('octicon-repo', 32)).toContain('width="32"');
});
