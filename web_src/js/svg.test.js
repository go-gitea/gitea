import {svg} from './svg.js';

test('svg', () => {
  expect(svg('octicon-repo')).toStartWith('<svg');
  expect(svg('octicon-repo', 16)).toInclude('width="16"');
  expect(svg('octicon-repo', 32)).toInclude('width="32"');
});
