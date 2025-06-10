import {createTippy} from '../modules/tippy.ts';
import {toggleElem} from '../utils/dom.ts';
import {registerGlobalEventFunc, registerGlobalInitFunc} from '../modules/observer.ts';
import $ from 'jquery';

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

window.addEventListener('DOMContentLoaded', () => {
  ($('input[name=history-enable-follow-renames]')[0] as HTMLInputElement).checked = location.toString().includes('history_follow_rename=true');
});

$('input[name=history-enable-follow-renames]').on('change', function() {
  const checked = ($(this)[0] as HTMLInputElement).matches(':checked');
  let url = location.toString();

  url = url.replaceAll(/history_follow_rename=(true|false)&*/g, '');
  if (url.endsWith('?')) {
    url = url.slice(0, -1);
  }
  if (url.includes('?')) {
    url += '&';
  } else {
    url += '?';
  }

  url += `history_follow_rename=${checked}`;
  window.location.href = url;
});
