import {registerGlobalEventFunc, registerGlobalInitFunc} from '../modules/observer.ts';

let nowrap = false;
export function initFileLinesWrap() {
  registerGlobalInitFunc('initFileLinesWrapButton', initFileLinesWrapButton);
  registerGlobalEventFunc('click', 'onToggleFileLinesWrapButtonClick', onToggleFileLinesWrapButtonClick);
  if (nowrap) {
    document.documentElement.classList.add('no-wrap-file-lines');
  }
}

function initFileLinesWrapButton(btn: HTMLElement) {
  btn.textContent = btn.getAttribute(nowrap ? 'data-wrap-text' : 'data-nowrap-text');
}

function onToggleFileLinesWrapButtonClick(btn: HTMLElement) {
  if (btn.classList.contains('disabled') || btn.classList.contains('is-loading')) return;
  nowrap = !nowrap;
  document.documentElement.classList.toggle('no-wrap-file-lines');
  btn.textContent = btn.getAttribute(nowrap ? 'data-wrap-text' : 'data-nowrap-text');
}
