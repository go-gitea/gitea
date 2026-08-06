import SvgIcon from './SvgIcon.vue';
import {createApp, h} from 'vue';

test('SvgIcon', () => {
  const root = document.createElement('div');
  createApp({render: () => h(SvgIcon, {name: 'octicon-link', size: 24, class: 'base'})}).mount(root);
  const node = root.firstChild as Element;
  expect(node.nodeName).toEqual('svg');
  expect(node.getAttribute('width')).toEqual('24');
  expect(node.getAttribute('height')).toEqual('24');
  expect(node.classList.contains('octicon-link')).toBeTruthy();
  expect(node.classList.contains('base')).toBeTruthy();
  expect(node.getAttribute('viewBox')).toEqual('0 0 16 16');
  expect(node.getAttribute('aria-hidden')).toEqual('true');
  expect(node.innerHTML).toContain('<path');
});

test('SvgIcon symbolId', () => {
  const root = document.createElement('div');
  createApp({render: () => h(SvgIcon, {name: 'octicon-rss', symbolId: 'svg-symbol-octicon-rss'})}).mount(root);
  const node = root.firstChild as Element;
  expect(node.classList.contains('svg-symbol-container')).toBeTruthy();
  expect(node.querySelector('symbol')!.getAttribute('id')).toEqual('svg-symbol-octicon-rss');
});
