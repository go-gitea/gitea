import {createTippy} from '../modules/tippy.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';

export function initRepoWatch() {
  registerGlobalInitFunc('initRepoWatchMenu', (btn: HTMLElement) => {
    const menu = btn.nextElementSibling!;
    const watchMenuTippy = createTippy(btn, {
      content: menu,
      theme: 'menu',
      maxWidth: 350,
      placement: 'bottom-end',
      trigger: 'click',
      interactive: true,
      hideOnClick: true,
    });
    menu.addEventListener('click', () => watchMenuTippy.hide());
  });
}
