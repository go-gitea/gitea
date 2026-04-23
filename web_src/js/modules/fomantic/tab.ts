import {queryElemSiblings} from '../../utils/dom.ts';

// clicking a `[data-tab]` menu item activates the matching `.ui.tab[data-tab=...]` panel
export function initTabs(elBtns: Iterable<Element>) {
  for (const elBtn of elBtns) {
    const tabName = elBtn.getAttribute('data-tab');
    if (!tabName) continue;
    const elTab = document.querySelector(`.ui.tab[data-tab="${tabName}"]`);
    if (!elTab) continue;
    elBtn.addEventListener('click', () => {
      queryElemSiblings(elTab, '.ui.tab', (el) => el.classList.remove('active'));
      queryElemSiblings(elBtn, '[data-tab]', (el) => el.classList.remove('active'));
      elBtn.classList.add('active');
      elTab.classList.add('active');
    });
  }
}
