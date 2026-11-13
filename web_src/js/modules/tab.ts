import {queryElemSiblings} from '../utils/dom.ts';

export function initTabSwitcher(tabItemContainer: Element) {
  for (const elItem of tabItemContainer.querySelectorAll('.item[data-tab]')) {
    const tabName = elItem.getAttribute('data-tab')!;
    elItem.addEventListener('click', () => {
      const elPanel = document.querySelector(`.ui.tab[data-tab="${CSS.escape(tabName)}"]`)!;
      queryElemSiblings(elPanel, '.ui.tab', (el) => el.classList.remove('active'));
      queryElemSiblings(elItem, '.item[data-tab]', (el) => el.classList.remove('active'));
      elItem.classList.add('active');
      elPanel.classList.add('active');
    });
  }
}
