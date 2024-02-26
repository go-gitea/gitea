import {throttle} from 'throttle-debounce';
import {svg} from '../svg.js';
import {createTippy} from '../modules/tippy.js';

const update = throttle(100, (menu) => {
  const menuItems = menu.querySelectorAll('.overflow-menu-items .item');
  const buttonItems = [];

  const {right: menuRight} = menu.getBoundingClientRect();
  for (const item of menuItems) {
    const {right: itemRight} = item.getBoundingClientRect();
    if (itemRight >= menuRight) {
      buttonItems.push(item.cloneNode(true));
      item.remove();
    }
  }

  if (buttonItems.length && !menu.querySelector('.overflow-menu-button')) {
    const btn = document.createElement('button');
    btn.classList.add('overflow-menu-button', 'btn', 'tw-px-2');
    btn.innerHTML = svg('octicon-kebab-horizontal');

    const itemsMenu = document.createElement('div');
    itemsMenu.classList.add('overflow-menu-tippy', 'ui', 'secondary', 'vertical', 'menu');
    for (const item of buttonItems) {
      itemsMenu.append(item);
    }

    createTippy(btn, {
      trigger: 'click',
      hideOnClick: true,
      interactive: true,
      placement: 'bottom-end',
      role: 'menu',
      content: itemsMenu,
    });

    menu.append(btn);
  }
});

export function initOverflowMenu() {
  for (const el of document.querySelectorAll('.overflow-menu')) {
    update(el);
    (new ResizeObserver((entries) => {
      // raf seems to avoid 'ResizeObserver loop completed with undelivered notifications' error
      requestAnimationFrame(() => {
        for (const entry of entries) {
          update(entry.target);
        }
      });
    })).observe(el);
  }
}
