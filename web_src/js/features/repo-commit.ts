import {createFloatingElement} from '../modules/floating.ts';
import {toggleElem} from '../utils/dom.ts';
import {registerGlobalEventFunc, registerGlobalInitFunc} from '../modules/observer.ts';

export function initRepoEllipsisButton() {
  registerGlobalEventFunc('click', 'onRepoEllipsisButtonClick', async (el: HTMLInputElement, e: Event) => {
    e.preventDefault();
    const expanded = el.getAttribute('aria-expanded') === 'true';
    toggleElem(el.parentElement!.querySelector('.commit-body')!);
    el.setAttribute('aria-expanded', String(!expanded));
  });
}

export function initCommitStatuses() {
  registerGlobalInitFunc('initCommitStatuses', (el: HTMLElement) => {
    const nextEl = el.nextElementSibling!;
    if (!nextEl.matches('.floating-target')) throw new Error('Expected next element to be a float target');
    createFloatingElement(el, {
      content: nextEl,
      placement: 'bottom-start',
      interactive: true,
      role: 'dialog',
      theme: 'box-with-header',
    });
  });
}
