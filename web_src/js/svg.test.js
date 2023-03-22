import {expect, test} from 'vitest';
import {svg, svgParseOuterInner} from './svg.js';

test('svg', () => {
  expect(svg('octicon-repo')).toMatch(/^<svg/);
  expect(svg('octicon-repo', 16)).toContain('width="16"');
  expect(svg('octicon-repo', 32)).toContain('width="32"');
});

test('svgParseOuterInner', () => {
  const {svgOuter, svgInnerHtml} = svgParseOuterInner('octicon-repo');
  expect(svgOuter.nodeName).toMatch('svg');
  expect(svgOuter.classList.contains('octicon-repo')).toBeTruthy();
  expect(svgInnerHtml).toContain('<path');
})
