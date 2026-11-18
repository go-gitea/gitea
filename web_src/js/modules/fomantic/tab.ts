import {queryElemSiblings} from '../../utils/dom.ts';

export function initTabSwitcher(tabItemContainer: Element) {
  // Clicking a `.item[data-tab]` menu item activates the nearest matching `.ui.tab[data-tab=...]` panel
  const tabItems = tabItemContainer.querySelectorAll('.item[data-tab]');
  for (const elItem of tabItems) {
    const tabName = elItem.getAttribute('data-tab')!;
    elItem.addEventListener('click', () => {
      const panelSelector = `.ui.tab[data-tab="${CSS.escape(tabName)}"]`;
      const elPanel = tabItemContainer.closest(`:has(${panelSelector})`)!.querySelector(panelSelector)!;
      queryElemSiblings(elPanel, '.ui.tab', (el) => el.classList.remove('active'));
      queryElemSiblings(elItem, '.item[data-tab]', (el) => el.classList.remove('active'));
      elItem.classList.add('active');
      elPanel.classList.add('active');
    });
  }
}
