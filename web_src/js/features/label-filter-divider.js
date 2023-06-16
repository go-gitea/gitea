import {toggleElem, onInputDebounce} from '../utils/dom.js';

export function initLabelSearchInput() {
  const menu = document.querySelector('.labels-filter-menu');
  if (!menu) return;
  // toggle dividers according to filtered results
  menu.querySelector('.labels-filter-input').addEventListener('input', onInputDebounce(() => {
    for (const divider of menu.querySelectorAll('[data-divider-index]')) {
      const dividerIndex = divider.getAttribute('data-divider-index');
      let showDivider = false;
      for (const el of menu.querySelectorAll(`[data-divider-group="${dividerIndex}"]`)) {
        if (!el.classList.contains('filtered')) {
          showDivider = true;
          break;
        }
      }
      toggleElem(divider, showDivider);
    }
  }));
}
