import {throttle} from 'throttle-debounce';
import {isHorizontallyOverflown, createElementFromHTML} from '../utils/dom.js';
import {svg} from '../svg.js';
import $ from 'jquery';

const update = throttle(100, (menu) => {
  const isOverflown = isHorizontallyOverflown(menu);
  if (isOverflown) {
    const overflownItems = [];
    const {right: menuRight} = menu.getBoundingClientRect();
    for (const item of menu.querySelectorAll('.overflow-menu-items .item')) {
      const {right: itemRight} = item.getBoundingClientRect();
      if (itemRight >= menuRight) {
        overflownItems.push(item.cloneNode(true));
        item.remove();
      }
    }
    if (overflownItems.length) {
      menu.querySelector('.overflow-menu')?.remove();
      menu.append(createElementFromHTML(`
        <div class="ui dropdown overflow-menu gt-px-2">
          <div class="text">${svg('octicon-kebab-horizontal')}</div>
          <div class="menu">${overflownItems.map((item) => item.outerHTML).join('')}</div>
        </div>
      `));
      $(menu).dropdown();
    }
  }
});

export function initOverflowMenu() {
  for (const el of document.querySelectorAll('.overflow-menu')) {
    update(el);
    (new ResizeObserver((entries) => {
      for (const entry of entries) {
        update(entry.target);
      }
    })).observe(el);
  }
}
