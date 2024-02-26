import {throttle} from 'throttle-debounce';
import {svg} from '../svg.js';
import {createTippy} from '../modules/tippy.js';

const update = throttle(100, (menu) => {
  let dropdownParent = menu.querySelector('.overflow-menu-dropdown');
  if (!dropdownParent) {
    const div = document.createElement('div');
    div.classList.add('overflow-menu-dropdown', 'ui', 'vertical', 'menu', 'tippy-target');
    menu.append(div);
    dropdownParent = div;
  }

  const menuParent = menu.querySelector('.overflow-menu-items');
  const dropdownItems = dropdownParent?.querySelectorAll('.item') || [];
  for (const item of dropdownItems) {
    menuParent.append(item);
  }

  // measure which items are outside the element boundary and move them into the button menu
  const itemsToMove = [];
  const menuRight = menu.parentNode.getBoundingClientRect().right;
  for (const item of menuParent.querySelectorAll('.item')) {
    const itemRight = item.getBoundingClientRect().right;
    if (menuRight - itemRight < 50) {
      itemsToMove.push(item);
    }
  }

  if (itemsToMove?.length) {
    for (const item of itemsToMove) {
      dropdownParent.append(item);
    }

    menu.querySelector('.overflow-menu-button')?.remove();
    const btn = document.createElement('button');
    btn.classList.add('overflow-menu-button', 'btn', 'tw-px-2');
    btn.innerHTML = svg('octicon-kebab-horizontal');
    menu.append(btn);

    createTippy(btn, {
      trigger: 'click',
      hideOnClick: true,
      interactive: true,
      placement: 'bottom-end',
      role: 'menu',
      content: dropdownParent.cloneNode(true),
    });
  } else {
    menu.querySelector('.overflow-menu-button')?.remove();
  }
});

export function initOverflowMenu() {
  for (const el of document.querySelectorAll('.overflow-menu')) {
    update(el);
    let lastSize;
    (new ResizeObserver((entries) => {
      for (const entry of entries) {
        const newSize = entry.contentBoxSize[0].inlineSize;
        if (newSize !== lastSize) { // only trigger update on horizontal size change
          update(entry.target);
          lastSize = newSize;
        }
      }
    })).observe(el);
  }
}
