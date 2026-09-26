import SvgIcon from './SvgIcon.vue';
import {createApp, h} from 'vue';

test('SvgIcon', () => {
  const root = document.createElement('div');
  createApp({render: () => h(SvgIcon, {name: 'octicon-dot-fill', size: 24, class: 'base', symbolId: 'svg-symbol-dot'})}).mount(root);
  expect(root.innerHTML).toBe(
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="24" height="24" aria-hidden="true" class="svg octicon-dot-fill tw-hidden svg-symbol-container base"><symbol id="svg-symbol-dot" viewBox="0 0 16 16"><path d="M8 4a4 4 0 1 1 0 8 4 4 0 0 1 0-8"></path></symbol></svg>`,
  );
});
