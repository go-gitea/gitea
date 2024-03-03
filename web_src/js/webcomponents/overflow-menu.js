import {throttle} from 'throttle-debounce';
import {createTippy} from '../modules/tippy.js';
import {isDocumentFragmentOrElementNode} from '../utils/dom.js';

window.customElements.define('overflow-menu', class extends HTMLElement {
  updateItems = throttle(100, () => {
    let tippyContent = this.querySelector('.overflow-menu-tippy-content');
    if (!tippyContent) {
      const div = document.createElement('div');
      div.classList.add('overflow-menu-tippy-content', 'ui', 'vertical', 'menu', 'tippy-target');
      this.append(div);
      tippyContent = div;
    }

    const tippyItems = tippyContent?.querySelectorAll('.item');
    for (const item of tippyItems) {
      this.menuItemsEl.append(item);
    }

    // measure which items are partially outside the element boundary and move them into the button menu
    const itemsToMove = [];
    const menuRight = this.getBoundingClientRect().right;
    for (const item of this.menuItemsEl.querySelectorAll('.item')) {
      const itemRight = item.getBoundingClientRect().right;
      if (menuRight - itemRight < 38) { // slightly less than width of .overflow-menu-button
        itemsToMove.push(item);
      }
    }

    if (itemsToMove?.length) {
      for (const item of itemsToMove) {
        tippyContent.append(item);
      }

      const content = tippyContent.cloneNode(true);
      const existingBtn = this.querySelector('.overflow-menu-button');
      if (existingBtn?._tippy) {
        existingBtn._tippy.setContent(content);
        return;
      }

      const btn = document.createElement('button');
      btn.classList.add('overflow-menu-button', 'btn', 'tw-px-2', 'hover:tw-text-text-dark');
      btn.innerHTML = '<svg viewBox="0 0 16 16" class="svg octicon-kebab-horizontal" width="16" height="16" aria-hidden="true"><path d="M8 9a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3M1.5 9a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3m13 0a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3"/></svg>';
      this.append(btn);

      createTippy(btn, {
        trigger: 'click',
        hideOnClick: true,
        interactive: true,
        placement: 'bottom-end',
        role: 'menu',
        content,
      });
    } else {
      const btn = this.querySelector('.overflow-menu-button');
      btn?._tippy?.destroy();
      btn?.remove();
    }
  });

  init() {
    // ResizeObserver seems to reliably trigger on page load, so we don't manually call updateItems()
    // for the initial rendering, which also avoids a full-page FOUC in Firefox that happens when
    // updateItems is called too soon.
    (new ResizeObserver((entries) => {
      for (const entry of entries) {
        const newWidth = entry.contentBoxSize[0].inlineSize;
        if (newWidth !== this.lastWidth) {
          this.updateItems();
          this.lastWidth = newWidth;
        }
      }
    })).observe(this);
  }

  connectedCallback() {
    // check whether the mandatory .overflow-menu-items child element is present initially which happens
    // with Vue which renders differently than browsers. If it's not there, like in the case of browser
    // template rendering, wait for its addition.
    const menuItemsEl = this.querySelector('.overflow-menu-items');
    if (menuItemsEl) {
      this.menuItemsEl = menuItemsEl;
      this.init();
    } else {
      const observer = new MutationObserver((mutations) => {
        for (const mutation of mutations) {
          for (const node of mutation.addedNodes) {
            if (!isDocumentFragmentOrElementNode(node)) continue;
            if (node.classList.contains('overflow-menu-items')) {
              this.menuItemsEl = node;
              observer?.disconnect();
              this.init();
            }
          }
        }
      });
      observer.observe(this, {childList: true});
    }
  }
});
