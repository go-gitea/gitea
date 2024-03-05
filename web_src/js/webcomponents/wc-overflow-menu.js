import {throttle} from 'throttle-debounce';
import {createTippy} from '../modules/tippy.js';
import {isDocumentFragmentOrElementNode} from '../utils/dom.js';
import octiconKebabHorizontal from '../../../public/assets/img/svg/octicon-kebab-horizontal.svg';

window.customElements.define('wc-overflow-menu', class extends HTMLElement {
  updateItems = throttle(100, () => {
    if (!this.tippyContent) {
      const div = document.createElement('div');
      div.classList.add('ui', 'vertical', 'menu', 'tippy-target');
      this.append(div);
      this.tippyContent = div;
    }

    // move items in tippy back into the menu items for subsequent measurement
    for (const item of this.tippyItems || []) {
      this.menuItemsEl.append(item);
    }

    // measure which items are partially outside the element and move them into the button menu
    this.tippyItems = [];
    const menuRight = this.offsetLeft + this.offsetWidth;
    for (const item of this.menuItemsEl.querySelectorAll('.item')) {
      const itemRight = item.offsetLeft + item.offsetWidth;
      if (menuRight - itemRight < 38) { // roughly the width of .wc-overflow-menu-button
        this.tippyItems.push(item);
      }
    }

    if (this.tippyItems?.length) {
      // move all items that overflow into tippy
      for (const item of this.tippyItems) {
        this.tippyContent.append(item);
      }

      // update existing tippy
      if (this.button?._tippy) {
        this.button._tippy.setContent(this.tippyContent);
        return;
      }

      const btn = document.createElement('button');
      btn.classList.add('wc-overflow-menu-button', 'btn', 'tw-px-2', 'hover:tw-text-text-dark');
      btn.innerHTML = octiconKebabHorizontal;
      this.append(btn);
      this.button = btn;

      createTippy(btn, {
        trigger: 'click',
        hideOnClick: true,
        interactive: true,
        placement: 'bottom-end',
        role: 'menu',
        content: this.tippyContent,
      });
    } else {
      const btn = this.querySelector('.wc-overflow-menu-button');
      btn?._tippy?.destroy();
      btn?.remove();
    }
  });

  init() {
    // ResizeObserver triggers on initial render, so we don't manually call `updateItems` here which
    // also avoids a full-page FOUC in Firefox that happens when `updateItems` is called too soon.
    this.resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const newWidth = entry.contentBoxSize[0].inlineSize;
        if (newWidth !== this.lastWidth) {
          requestAnimationFrame(() => {
            this.updateItems();
          });
          this.lastWidth = newWidth;
        }
      }
    });
    this.resizeObserver.observe(this);
  }

  connectedCallback() {
    // check whether the mandatory `.wc-overflow-menu-items` element is present initially which happens
    // with Vue which renders differently than browsers. If it's not there, like in the case of browser
    // template rendering, wait for its addition.
    const menuItemsEl = this.querySelector('.wc-overflow-menu-items');
    if (menuItemsEl) {
      this.menuItemsEl = menuItemsEl;
      this.init();
    } else {
      this.mutationObserver = new MutationObserver((mutations) => {
        for (const mutation of mutations) {
          for (const node of mutation.addedNodes) {
            if (!isDocumentFragmentOrElementNode(node)) continue;
            if (node.classList.contains('wc-overflow-menu-items')) {
              this.menuItemsEl = node;
              this.mutationObserver?.disconnect();
              this.init();
            }
          }
        }
      });
      this.mutationObserver.observe(this, {childList: true});
    }
  }

  disconnectedCallback() {
    this.mutationObserver?.disconnect();
    this.resizeObserver?.disconnect();
  }
});
