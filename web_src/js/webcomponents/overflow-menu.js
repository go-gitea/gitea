import {throttle} from 'throttle-debounce';
import {createTippy} from '../modules/tippy.js';
import {isDocumentFragmentOrElementNode} from '../utils/dom.js';
import octiconKebabHorizontal from '../../../public/assets/img/svg/octicon-kebab-horizontal.svg';

window.customElements.define('overflow-menu', class extends HTMLElement {
  updateItems = throttle(100, () => {
    if (!this.tippyContent) {
      const div = document.createElement('div');
      div.classList.add('tippy-target');
      div.tabIndex = '-1'; // for initial focus, programmatic focus only
      div.addEventListener('keydown', (e) => {
        if (e.key === 'Tab') {
          const items = this.tippyContent.querySelectorAll('[role="menuitem"]');
          if (e.shiftKey) {
            if (document.activeElement === items[0]) {
              e.preventDefault();
              items[items.length - 1].focus();
            }
          } else {
            if (document.activeElement === items[items.length - 1]) {
              e.preventDefault();
              items[0].focus();
            }
          }
        } else if (e.key === 'Escape') {
          e.preventDefault();
          e.stopPropagation();
          this.button._tippy.hide();
          this.button.focus();
        } else if (e.key === ' ' || e.code === 'Enter') {
          if (document.activeElement?.matches('[role="menuitem"]')) {
            e.preventDefault();
            e.stopPropagation();
            document.activeElement.click();
          }
        } else if (e.key === 'ArrowDown') {
          if (document.activeElement?.matches('.tippy-target')) {
            e.preventDefault();
            e.stopPropagation();
            document.activeElement.querySelector('[role="menuitem"]:first-of-type').focus();
          } else if (document.activeElement?.matches('[role="menuitem"]')) {
            e.preventDefault();
            e.stopPropagation();
            document.activeElement.nextElementSibling?.focus();
          }
        } else if (e.key === 'ArrowUp') {
          if (document.activeElement?.matches('.tippy-target')) {
            e.preventDefault();
            e.stopPropagation();
            document.activeElement.querySelector('[role="menuitem"]:last-of-type').focus();
          } else if (document.activeElement?.matches('[role="menuitem"]')) {
            e.preventDefault();
            e.stopPropagation();
            document.activeElement.previousElementSibling?.focus();
          }
        }
      });
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
    const menuItems = this.menuItemsEl.querySelectorAll('.item');
    for (const item of menuItems) {
      const itemRight = item.offsetLeft + item.offsetWidth;
      if (menuRight - itemRight < 38) { // roughly the width of .overflow-menu-button
        this.tippyItems.push(item);
      }
    }

    // if there are no overflown items, remove any previously created button
    if (!this.tippyItems?.length) {
      const btn = this.querySelector('.overflow-menu-button');
      btn?._tippy?.destroy();
      btn?.remove();
      return;
    }

    // remove aria role from items that moved from tippy to menu
    for (const item of menuItems) {
      if (!this.tippyItems.includes(item)) {
        item.removeAttribute('role');
      }
    }

    // move all items that overflow into tippy
    for (const item of this.tippyItems) {
      item.setAttribute('role', 'menuitem');
      this.tippyContent.append(item);
    }

    // update existing tippy
    if (this.button?._tippy) {
      this.button._tippy.setContent(this.tippyContent);
      return;
    }

    // create button initially
    const btn = document.createElement('button');
    btn.classList.add('overflow-menu-button', 'btn', 'tw-px-2', 'hover:tw-text-text-dark');
    btn.setAttribute('aria-label', window.config.i18n.more_items);
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
      onShow: () => { // FIXME: onShown doesn't work (never be called)
        setTimeout(() => {
          this.tippyContent.focus();
        }, 0);
      },
    });
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
    this.setAttribute('role', 'navigation');

    // check whether the mandatory `.overflow-menu-items` element is present initially which happens
    // with Vue which renders differently than browsers. If it's not there, like in the case of browser
    // template rendering, wait for its addition.
    // The eslint rule is not sophisticated enough or aware of this problem, see
    // https://github.com/43081j/eslint-plugin-wc/pull/130
    const menuItemsEl = this.querySelector('.overflow-menu-items'); // eslint-disable-line wc/no-child-traversal-in-connectedcallback
    if (menuItemsEl) {
      this.menuItemsEl = menuItemsEl;
      this.init();
    } else {
      this.mutationObserver = new MutationObserver((mutations) => {
        for (const mutation of mutations) {
          for (const node of mutation.addedNodes) {
            if (!isDocumentFragmentOrElementNode(node)) continue;
            if (node.classList.contains('overflow-menu-items')) {
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
