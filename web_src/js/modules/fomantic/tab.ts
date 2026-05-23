import {queryElemSiblings} from '../../utils/dom.ts';

export function initTabSwitcher(tabItemContainer: Element) {
  // Clicking a `.item[data-tab]` menu item activates the matching `.ui.tab[data-tab=...]` panel
  // This design is from Fomantic UI, and it has problems like :
  // * The panel selector is global, callers should make sure the "data-tab" values don't conflict on the same page
  const tabItems = tabItemContainer.querySelectorAll('.item[data-tab]');
  for (const elItem of tabItems) {
    const tabName = elItem.getAttribute('data-tab')!;
    elItem.addEventListener('click', () => {
      const elPanel = document.querySelector(`.ui.tab[data-tab="${tabName}"]`)!;
      queryElemSiblings(elPanel, '.ui.tab', (el) => el.classList.remove('active'));
      queryElemSiblings(elItem, '.item[data-tab]', (el) => el.classList.remove('active'));
      elItem.classList.add('active');
      elPanel.classList.add('active');
    });
  }
}
