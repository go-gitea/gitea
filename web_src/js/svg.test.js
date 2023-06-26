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
  const node = root.firstChild;
  expect(node.nodeName).toEqual('svg');
  expect(node.getAttribute('width')).toEqual('24');
  expect(node.getAttribute('height')).toEqual('24');
  expect(node.classList.contains('octicon-link')).toBeTruthy();
  expect(node.classList.contains('base')).toBeTruthy();
  expect(node.classList.contains('extra')).toBeTruthy();
});
