import {throttle} from '../utils/func.ts';
import {createElementFromAttrs} from '../utils/dom.ts';
import octiconKebabHorizontal from '../../../public/assets/img/svg/octicon-kebab-horizontal.svg';

window.customElements.define('overflow-menu', class extends HTMLElement {
  dropdown?: {container: HTMLElement, button: HTMLButtonElement, popup: HTMLElement};
  overflowItems: Array<{item: HTMLElement, tabIndex: string | null, hadRole: boolean, afterSpacer: boolean}> = [];
  menuItemsEl!: HTMLElement;
  resizeObserver!: ResizeObserver;
  mutationObserver!: MutationObserver;
  lastWidth!: number;

  updateButtonActivationState() {
    if (!this.dropdown) return;
    this.dropdown.button.classList.toggle('active', Boolean(this.dropdown.popup.querySelector('.item.active')));
  }

  handleItemClick = (e: MouseEvent) => {
    const item = (e.target as Element).closest('.item');
    if (!item || item.parentElement !== e.currentTarget || !this.dropdown) return;
    if (item.matches('.active[data-tab]')) {
      const otherItems = item.parentElement === this.dropdown.popup ? this.menuItemsEl : this.dropdown.popup;
      for (const active of otherItems.querySelectorAll(':scope > .item.active[data-tab]')) active.classList.remove('active');
    }
    queueMicrotask(() => { // Vue replaces the dashboard filters' classes when selection changes.
      for (const {item} of this.overflowItems) item.classList.add('dropdown-item');
      this.updateButtonActivationState();
    });
  };

  updateItems = throttle(() => {
    const spacer = this.menuItemsEl.querySelector(':scope > .item-flex-space');
    for (const {item, tabIndex, hadRole, afterSpacer} of this.overflowItems) {
      item.classList.remove('dropdown-item');
      if (item.toggleAttribute('tabindex', tabIndex !== null)) item.setAttribute('tabindex', tabIndex!);
      if (!hadRole) item.removeAttribute('role');
      if (!spacer || afterSpacer) {
        this.menuItemsEl.append(item);
      } else {
        spacer.before(item);
      }
    }

    spacer?.classList.add('tw-hidden');
    this.menuItemsEl.classList.remove('overflow-menu-has-overflow');
    this.overflowItems = [];
    const menuWidth = this.clientWidth;
    const menuItems = [...this.menuItemsEl.querySelectorAll<HTMLElement>(':scope > .item, :scope > .item-flex-space')];
    const spacerIndex = menuItems.findIndex((item) => item === spacer);
    for (const [index, item] of menuItems.entries()) {
      if (item.classList.contains('item-flex-space')) continue;
      const itemRight = item.offsetLeft + item.offsetWidth;
      const lastItemFits = index === menuItems.length - 1 && !this.overflowItems.length && itemRight < menuWidth;
      if (menuWidth - itemRight < 38 && !lastItemFits) {
        this.overflowItems.push({
          item, afterSpacer: spacerIndex >= 0 && index > spacerIndex,
          tabIndex: item.getAttribute('tabindex'), hadRole: item.hasAttribute('role'),
        });
      }
    }
    spacer?.classList.remove('tw-hidden');

    if (!this.overflowItems.length) {
      this.dropdown?.container.remove();
      return;
    }

    if (!this.dropdown) {
      const button = createElementFromAttrs<HTMLButtonElement>('button', {
        type: 'button', class: 'dropdown-trigger overflow-menu-button', 'aria-label': window.config.i18n.more_items,
      });
      button.innerHTML = octiconKebabHorizontal;
      const popup = createElementFromAttrs('div', {class: 'dropdown-popup dropdown-popup-flex'});
      popup.addEventListener('click', this.handleItemClick);
      const container = createElementFromAttrs('div', {class: 'dropdown-container', 'data-placement': 'bottom-end'}, button, popup);
      this.dropdown = {container, button, popup};
    }

    for (const {item} of this.overflowItems) {
      item.classList.add('dropdown-item');
      this.dropdown.popup.append(item);
    }
    if (!this.dropdown.container.parentElement) {
      this.menuItemsEl.append(this.dropdown.container);
    }
    this.menuItemsEl.classList.add('overflow-menu-has-overflow');
    this.updateButtonActivationState();
  }, 100);

  init() {
    const menuItemsEl = this.querySelector<HTMLElement>('.overflow-menu-items');
    if (!menuItemsEl) return false;
    this.menuItemsEl = menuItemsEl;

    // Reserve the width of active text so changing tabs does not resize the menu.
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
            this.setAttribute('data-ready', ''); // reveal via CSS [data-ready]
          });
          this.lastWidth = newWidth;
        }
      }
    });
    this.resizeObserver.observe(this);
    this.menuItemsEl.addEventListener('click', this.handleItemClick);
    return true;
  }

  connectedCallback() {
    this.setAttribute('role', 'navigation');
    if (this.init()) return;
    this.mutationObserver = new MutationObserver(() => {
      if (this.init()) this.mutationObserver.disconnect();
    });
    this.mutationObserver.observe(this, {childList: true});
  }

  disconnectedCallback() {
    this.mutationObserver?.disconnect();
    this.resizeObserver?.disconnect();
  }
});
