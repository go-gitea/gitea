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

    const menuParent = this.querySelector('.overflow-menu-items');
    const dropdownItems = tippyContent?.querySelectorAll('.item');
    for (const item of dropdownItems) {
      menuParent.append(item);
    }

    // measure which items are partially outside the element boundary and move them into the button menu
    const itemsToMove = [];
    const menuRight = this.offsetLeft + this.offsetWidth;
    for (const item of menuParent.querySelectorAll('.item')) {
      const itemRight = item.offsetLeft + item.offsetWidth;
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
    this.updateItems();
    let lastWidth;
    (new ResizeObserver((entries) => {
      for (const entry of entries) {
        const newWidth = entry.contentBoxSize[0].inlineSize;
        if (newWidth !== lastWidth) {
          this.updateItems();
          lastWidth = newWidth;
        }
      }
    })).observe(this);
  }

  connectedCallback() {
    // check whether the mandatory .overflow-menu-items child element is present initially which can
    // happen when used with Vue. If it's not there, wait for its addition via Mutationobserver, which
    // is generally what happens when rendered via templates.
    if (this.querySelector('.overflow-menu-items')) {
      this.init();
    } else {
      const observer = new MutationObserver((mutations) => {
        for (const mutation of mutations) {
          for (const node of mutation.addedNodes) {
            if (!isDocumentFragmentOrElementNode(node)) continue;
            if (node.classList.contains('overflow-menu-items')) {
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
