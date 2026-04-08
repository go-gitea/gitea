import {throttle} from 'throttle-debounce';
import {addDelegatedEventListener, generateElemId, isDocumentFragmentOrElementNode} from '../utils/dom.ts';
import octiconKebabHorizontal from '../../../public/assets/img/svg/octicon-kebab-horizontal.svg';

window.customElements.define('overflow-menu', class extends HTMLElement {
  popup: HTMLDivElement;
  overflowItems: Array<HTMLElement>;
  button: HTMLButtonElement | null;
  menuItemsEl: HTMLElement;
  resizeObserver: ResizeObserver;
  mutationObserver: MutationObserver;
  lastWidth: number;

  updateButtonActivationState() {
    if (!this.button || !this.popup) return;
    this.button.classList.toggle('active', Boolean(this.popup.querySelector('.item.active')));
  }

  showPopup() {
    if (!this.popup || this.popup.style.display !== 'none') return;
    this.popup.style.display = '';
    this.button!.setAttribute('aria-expanded', 'true');
    setTimeout(() => this.popup.focus(), 0);
    document.addEventListener('click', this.onClickOutside, true);
  }

  hidePopup() {
    if (!this.popup || this.popup.style.display === 'none') return;
    this.popup.style.display = 'none';
    this.button?.setAttribute('aria-expanded', 'false');
    document.removeEventListener('click', this.onClickOutside, true);
  }

  onClickOutside = (e: Event) => {
    if (!this.popup?.contains(e.target as Node) && !this.button?.contains(e.target as Node)) {
      this.hidePopup();
    }
  };

  updateItems = throttle(100, () => {
    if (!this.popup) {
      const div = document.createElement('div');
      div.classList.add('overflow-menu-popup');
      div.setAttribute('role', 'menu');
      div.tabIndex = -1; // for initial focus, programmatic focus only
      div.style.display = 'none';
      div.addEventListener('keydown', (e) => {
        if (e.isComposing) return;
        if (e.key === 'Tab') {
          const items = this.popup.querySelectorAll<HTMLElement>('[role="menuitem"]');
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
          this.hidePopup();
          this.button?.focus();
        } else if (e.key === ' ' || e.code === 'Enter') {
          if (document.activeElement?.matches('[role="menuitem"]')) {
            e.preventDefault();
            e.stopPropagation();
            (document.activeElement as HTMLElement).click();
          }
        } else if (e.key === 'ArrowDown') {
          if (document.activeElement === this.popup) {
            e.preventDefault();
            e.stopPropagation();
            this.popup.querySelector<HTMLElement>('[role="menuitem"]:first-of-type')?.focus();
          } else if (document.activeElement?.matches('[role="menuitem"]')) {
            e.preventDefault();
            e.stopPropagation();
            (document.activeElement.nextElementSibling as HTMLElement)?.focus();
          }
        } else if (e.key === 'ArrowUp') {
          if (document.activeElement === this.popup) {
            e.preventDefault();
            e.stopPropagation();
            this.popup.querySelector<HTMLElement>('[role="menuitem"]:last-of-type')?.focus();
          } else if (document.activeElement?.matches('[role="menuitem"]')) {
            e.preventDefault();
            e.stopPropagation();
            (document.activeElement.previousElementSibling as HTMLElement)?.focus();
          }
        }
      });
      this.handleItemClick(div, '.overflow-menu-popup > .item');
      this.popup = div;
    } // end if: no popup and create a new one

    const itemFlexSpace = this.menuItemsEl.querySelector<HTMLSpanElement>('.item-flex-space');
    const itemOverFlowMenuButton = this.querySelector<HTMLButtonElement>('.overflow-menu-button');

    // move items in popup back into the menu items for subsequent measurement
    for (const item of this.overflowItems || []) {
      if (!itemFlexSpace || item.getAttribute('data-after-flex-space')) {
        this.menuItemsEl.append(item);
      } else {
        itemFlexSpace.insertAdjacentElement('beforebegin', item);
      }
    }

    // measure which items are partially outside the element and move them into the button menu
    // flex space and overflow menu are excluded from measurement
    itemFlexSpace?.style.setProperty('display', 'none', 'important');
    itemOverFlowMenuButton?.style.setProperty('display', 'none', 'important');
    this.overflowItems = [];
    const menuRight = this.offsetLeft + this.offsetWidth;
    const menuItems = this.menuItemsEl.querySelectorAll<HTMLElement>('.item, .item-flex-space');
    let afterFlexSpace = false;
    for (const [idx, item] of menuItems.entries()) {
      if (item.classList.contains('item-flex-space')) {
        afterFlexSpace = true;
        continue;
      }
      if (afterFlexSpace) item.setAttribute('data-after-flex-space', 'true');
      const itemRight = item.offsetLeft + item.offsetWidth;
      if (menuRight - itemRight < 38) { // roughly the width of .overflow-menu-button with some extra space
        const onlyLastItem = idx === menuItems.length - 1 && this.overflowItems.length === 0;
        const lastItemFit = onlyLastItem && menuRight - itemRight > 0;
        const moveToPopup = !onlyLastItem || !lastItemFit;
        if (moveToPopup) this.overflowItems.push(item);
      }
    }
    itemFlexSpace?.style.removeProperty('display');
    itemOverFlowMenuButton?.style.removeProperty('display');

    // if there are no overflown items, remove any previously created button
    if (!this.overflowItems?.length) {
      this.hidePopup();
      this.button?.remove();
      this.popup?.remove();
      this.button = null;
      return;
    }

    // remove aria role from items that moved from popup to menu
    for (const item of menuItems) {
      if (!this.overflowItems.includes(item)) {
        item.removeAttribute('role');
      }
    }

    // move all items that overflow into popup
    for (const item of this.overflowItems) {
      item.setAttribute('role', 'menuitem');
      this.popup.append(item);
    }

    // update existing popup
    if (this.button) {
      this.updateButtonActivationState();
      return;
    }

    // create button and attach popup
    const popupId = generateElemId('overflow-popup-');
    this.popup.id = popupId;

    this.button = document.createElement('button');
    this.button.classList.add('overflow-menu-button');
    this.button.setAttribute('aria-label', window.config.i18n.more_items);
    this.button.setAttribute('aria-haspopup', 'true');
    this.button.setAttribute('aria-expanded', 'false');
    this.button.setAttribute('aria-controls', popupId);
    this.button.innerHTML = octiconKebabHorizontal;
    this.button.addEventListener('click', (e) => {
      e.stopPropagation();
      if (this.popup.style.display === 'none') {
        this.showPopup();
      } else {
        this.hidePopup();
      }
    });
    this.append(this.button);
    this.append(this.popup);
    this.updateButtonActivationState();
  });

  init() {
    // for horizontal menus where fomantic boldens active items, prevent this bold text from
    // enlarging the menu's active item replacing the text node with a div that renders a
    // invisible pseudo-element that enlarges the box.
    if (this.matches('.ui.secondary.pointing.menu, .ui.tabular.menu')) {
      for (const item of this.querySelectorAll('.item')) {
        for (const child of item.childNodes) {
          if (child.nodeType === Node.TEXT_NODE) {
            const text = child.textContent?.trim(); // whitespace is insignificant inside flexbox
            if (!text) continue;
            const span = document.createElement('span');
            span.classList.add('resize-for-semibold');
            span.setAttribute('data-text', text);
            span.textContent = text;
            child.replaceWith(span);
          }
        }
      }
    }

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
    this.handleItemClick(this, '.overflow-menu-items > .item');
  }

  handleItemClick(el: Element, selector: string) {
    addDelegatedEventListener(el, 'click', selector, () => {
      this.hidePopup();
      this.updateButtonActivationState();
    });
  }

  connectedCallback() {
    this.setAttribute('role', 'navigation');

    // check whether the mandatory `.overflow-menu-items` element is present initially which happens
    // with Vue which renders differently than browsers. If it's not there, like in the case of browser
    // template rendering, wait for its addition.
    // The eslint rule is not sophisticated enough or aware of this problem, see
    // https://github.com/43081j/eslint-plugin-wc/pull/130
    const menuItemsEl = this.querySelector<HTMLElement>('.overflow-menu-items'); // eslint-disable-line wc/no-child-traversal-in-connectedcallback
    if (menuItemsEl) {
      this.menuItemsEl = menuItemsEl;
      this.init();
    } else {
      this.mutationObserver = new MutationObserver((mutations) => {
        for (const mutation of mutations) {
          for (const node of mutation.addedNodes as NodeListOf<HTMLElement>) {
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
    document.removeEventListener('click', this.onClickOutside, true);
  }
});
