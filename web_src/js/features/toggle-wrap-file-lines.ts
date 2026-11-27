import {registerGlobalEventFunc, registerGlobalInitFunc} from '../modules/observer.ts';
import {localUserSettings} from '../modules/user-settings.ts';

const LOCAL_STORAGE_KEY = 'no_wrap_file_lines';

export function initFileLinesWrap() {
  const nowrap = localUserSettings.getBoolean(LOCAL_STORAGE_KEY, false);
  if (nowrap) {
    document.documentElement.classList.add('no-wrap-file-lines');
  }
  registerGlobalInitFunc('initFileLinesWrapButton', initFileLinesWrapButton);
  registerGlobalEventFunc('click', 'onToggleFileLinesWrapButtonClick', onToggleFileLinesWrapButtonClick);
}

function initFileLinesWrapButton(btn: HTMLElement) {
  const nowrap = localUserSettings.getBoolean(LOCAL_STORAGE_KEY, false);
  btn.textContent = btn.getAttribute(nowrap ? 'data-wrap-text' : 'data-nowrap-text');
}

function onToggleFileLinesWrapButtonClick(btn: HTMLElement) {
  if (btn.classList.contains('disabled') || btn.classList.contains('is-loading')) return;
  const new_nowrap = !localUserSettings.getBoolean(LOCAL_STORAGE_KEY);
  localUserSettings.setBoolean(LOCAL_STORAGE_KEY, new_nowrap);
  document.documentElement.classList.toggle('no-wrap-file-lines', new_nowrap);
  btn.textContent = btn.getAttribute(new_nowrap ? 'data-wrap-text' : 'data-nowrap-text');
}
