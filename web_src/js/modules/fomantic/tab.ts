import $ from 'jquery';
import {queryElemSiblings} from '../../utils/dom.ts';

export function initFomanticTab() {
  $.fn.tab = function (this: any, arg0: any) {
    const autoTabActivation = arg0?.autoTabActivation;
    for (const elBtn of this) {
      const tabName = elBtn.getAttribute('data-tab');
      if (!tabName) continue;
      elBtn.addEventListener('click', () => {
        const elTab = document.querySelector(`.ui.tab[data-tab="${tabName}"]`);
        queryElemSiblings(elTab, `.ui.tab`, (el) => el.classList.remove('active'));
        queryElemSiblings(elBtn, `[data-tab]`, (el) => el.classList.remove('active'));
        elBtn.classList.add('active');
        elTab.classList.add('active');
      });
      if (autoTabActivation && elBtn.classList.contains('active')) {
        const elTab = elBtn.querySelector(`.ui.tab[data-tab="${tabName}"]`);
        elTab.classList.add('active');
      }
    }
    return this;
  };
}
