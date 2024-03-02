import {throttle} from 'throttle-debounce';
import {svg} from '../svg.js';
import {createTippy} from '../modules/tippy.js';

const update = throttle(100, (overflowMenu) => {
  let dropdownParent = overflowMenu.querySelector('.overflow-menu-dropdown');
  if (!dropdownParent) {
    const div = document.createElement('div');
    div.classList.add('overflow-menu-dropdown', 'ui', 'vertical', 'menu', 'tippy-target');
    overflowMenu.append(div);
    dropdownParent = div;
  }

  const menuParent = overflowMenu.querySelector('.overflow-menu-items');
  const dropdownItems = dropdownParent?.querySelectorAll('.item') || [];
  for (const item of dropdownItems) {
    menuParent.append(item);
  }

  // measure which items are outside the element boundary and move them into the button menu
  const itemsToMove = [];
  const menuRight = overflowMenu.parentNode.getBoundingClientRect().right;
  for (const item of menuParent.querySelectorAll('.item')) {
    const itemRight = item.getBoundingClientRect().right;
    if (menuRight - itemRight < 22) { // slightly less than width of .overflow-menu-button
      itemsToMove.push(item);
    }
  }

  if (itemsToMove?.length) {
    for (const item of itemsToMove) {
      dropdownParent.append(item);
    }

    overflowMenu.querySelector('.overflow-menu-button')?.remove();
    const btn = document.createElement('button');
    btn.classList.add('overflow-menu-button', 'btn', 'tw-px-2');
    btn.innerHTML = svg('octicon-kebab-horizontal');
    overflowMenu.append(btn);

    createTippy(btn, {
      trigger: 'click',
      hideOnClick: true,
      interactive: true,
      placement: 'bottom-end',
      role: 'menu',
      content: dropdownParent.cloneNode(true),
    });
  } else {
    overflowMenu.querySelector('.overflow-menu-button')?.remove();
  }
});

window.customElements.define('overflow-menu', class extends HTMLElement {
  connectedCallback() {
    // raf is needed, otherwise the first update will no see all children
    requestAnimationFrame(() => {
      update(this);
      let lastWidth;
      (new ResizeObserver((entries) => {
        for (const entry of entries) {
          const newWidth = entry.contentBoxSize[0].inlineSize;
          if (newWidth !== lastWidth) {
            update(entry.target);
            lastWidth = newWidth;
          }
        }
      })).observe(this);
    });
  }
});
