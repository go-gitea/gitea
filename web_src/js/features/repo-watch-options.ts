// import {fomanticQuery} from '../modules/fomantic/base.ts';
import {createTippy} from '../modules/tippy.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
// import {submitFormFetchAction} from './common-fetch-action.ts';

export function initRepoWatchOptions() {
  registerGlobalInitFunc('initWatchOptions', (el: HTMLElement) => {
    const popup = el.lastElementChild!;
    if (!popup.matches('.tippy-target')) throw new Error('Expected last child to be a tippy target');
    createTippy(el, {
      content: popup,
      placement: 'bottom-end',
      trigger: 'click',
      interactive: true,
      hideOnClick: true,
      role: 'dialog',
      theme: 'default',
    });
  });
}
