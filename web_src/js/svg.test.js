import {expect, test} from 'vitest';
import {svg, SvgIcon, svgParseOuterInner} from './svg.js';
import {createApp, h} from 'vue';

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
});

test('SvgIcon', () => {
  const root = document.createElement('div');
  createApp({render: () => h(SvgIcon, {name: 'octicon-link', size: 24, class: 'base', className: 'extra'})}).mount(root);
  expect(root.firstChild.nodeName).toEqual('svg');
  expect(root.firstChild.getAttribute('width')).toEqual('24');
  expect(root.firstChild.getAttribute('height')).toEqual('24');
  expect(root.firstChild.classList.contains('octicon-link')).toBeTruthy();
  expect(root.firstChild.classList.contains('base')).toBeTruthy();
  expect(root.firstChild.classList.contains('extra')).toBeTruthy();
});
