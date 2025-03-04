import {createTippy} from '../modules/tippy.ts';
import {toggleElem} from '../utils/dom.ts';
import {registerGlobalEventFunc} from '../modules/observer.ts';

export function initRepoEllipsisButton() {
  registerGlobalEventFunc('click', 'onRepoEllipsisButtonClick', async (el: HTMLInputElement, e: Event) => {
    e.preventDefault();
    const expanded = el.getAttribute('aria-expanded') === 'true';
    toggleElem(el.parentElement.querySelector('.commit-body'));
    el.setAttribute('aria-expanded', String(!expanded));
  });
}

export function initCommitStatuses() {
  for (const element of document.querySelectorAll('[data-tippy="commit-statuses"]')) {
    const top = document.querySelector('.repository.file.list') || document.querySelector('.repository.diff');

    createTippy(element, {
      content: element.nextElementSibling,
      placement: top ? 'top-start' : 'bottom-start',
      interactive: true,
      role: 'dialog',
      theme: 'box-with-header',
    });
  }
}
