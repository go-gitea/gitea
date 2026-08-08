import {createTippy} from '../modules/tippy.ts';
import {showFomanticModal} from '../modules/fomantic/modal.ts';
import {registerGlobalEventFunc, registerGlobalInitFunc} from '../modules/observer.ts';
import type {Instance} from 'tippy.js';

let watchMenuTippy: Instance | null = null;

export function initRepoWatch() {
  registerGlobalInitFunc('initRepoWatchMenu', (btn: HTMLElement) => {
    watchMenuTippy?.destroy(); // a watch action replaces the button, orphaning the old menu
    const menu = btn.nextElementSibling!;
    watchMenuTippy = createTippy(btn, {
      content: menu,
      theme: 'menu',
      maxWidth: 350,
      placement: 'bottom-end',
      trigger: 'click',
      interactive: true,
      hideOnClick: true,
    });
    menu.addEventListener('click', () => watchMenuTippy!.hide());
  });
}
