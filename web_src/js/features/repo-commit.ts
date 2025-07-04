import {createTippy} from '../modules/tippy.ts';
import {toggleElem} from '../utils/dom.ts';
import {registerGlobalEventFunc, registerGlobalInitFunc} from '../modules/observer.ts';

export function initRepoEllipsisButton() {
  registerGlobalEventFunc('click', 'onRepoEllipsisButtonClick', async (el: HTMLInputElement, e: Event) => {
    e.preventDefault();
    const expanded = el.getAttribute('aria-expanded') === 'true';
    toggleElem(el.parentElement.querySelector('.commit-body'));
    el.setAttribute('aria-expanded', String(!expanded));
  });
}

export function initCommitStatuses() {
  registerGlobalInitFunc('initCommitStatuses', (el: HTMLElement) => {
    const nextEl = el.nextElementSibling;
    if (!nextEl.matches('.tippy-target')) throw new Error('Expected next element to be a tippy target');
    createTippy(el, {
      content: nextEl,
      placement: 'bottom-start',
      interactive: true,
      role: 'dialog',
      theme: 'box-with-header',
    });
  });
}

export function initCommitFileHistoryFollowRename() {
  const checkbox : HTMLInputElement | null = document.querySelector('input[name=history-enable-follow-renames]');

  if (!checkbox) {
    return;
  }
  const url = new URL(window.location.toString());
  checkbox.checked = url.searchParams.has('history_follow_rename', 'true');

  checkbox.addEventListener('change', () => {
    const url = new URL(window.location);

    url.searchParams.set('history_follow_rename', `${checkbox.checked}`);
    window.location.replace(url);
  });
}
