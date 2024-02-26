import {debounce} from 'throttle-debounce';
import {svg} from '../svg.js';
import {createTippy} from '../modules/tippy.js';

const update = debounce(250, (menu) => {
  const menuParent = menu.querySelector('.overflow-menu-items');
  let buttonParent = menu.querySelector('.overflow-menu-button-items');
  const menuItems = menuParent.querySelectorAll('.item');

  if (!buttonParent) {
    const div = document.createElement('div');
    div.classList.add('overflow-menu-button-items', 'ui', 'secondary', 'vertical', 'menu', 'tippy-target');
    menu.append(div);
    buttonParent = div;
  }

  const buttonItems = buttonParent?.querySelectorAll('.item') || [];

  for (const item of buttonItems) {
    menuParent.append(item);
  }

  // measure which items are outside the element boundary and move them into the button menu
  const itemsToMove = [];
  const {right: menuRight} = menu.parentNode.getBoundingClientRect();
  for (const item of menuItems) {
    const {right: itemRight} = item.getBoundingClientRect();
    if (menuRight - itemRight < 40) {
      itemsToMove.push(item);
    }
  }

  if (itemsToMove?.length) {
    for (const item of itemsToMove) {
      buttonParent.append(item);
    }

    if (!menu.querySelector('.overflow-menu-button')) {
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
        content: buttonParent.cloneNode(true),
      });
    }
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
