import {showInfoToast, showWarningToast, showErrorToast} from './toast.ts';
import type {Toast} from './toast.ts';
import {registerGlobalInitFunc} from './observer.ts';
import {showFomanticModal} from './fomantic/modal.ts';
import {createElementFromHTML} from '../utils/dom.ts';
import {html} from '../utils/html.ts';
import {showGlobalErrorMessage} from './errors.ts';

type LevelMap = Record<string, (message: string) => Toast | null>;

function initDevtestPage() {
  const toastButtons = document.querySelectorAll('.toast-test-button');
  if (toastButtons.length) {
    const levelMap: LevelMap = {info: showInfoToast, warning: showWarningToast, error: showErrorToast};
    for (const el of toastButtons) {
      el.addEventListener('click', () => {
        const level = el.getAttribute('data-toast-level')!;
        const message = el.getAttribute('data-toast-message')!;
        levelMap[level](message);
      });
    }
  }

  const modalButtons = document.querySelector('.modal-buttons');
  if (modalButtons) {
    for (const el of document.querySelectorAll('.ui.modal:not([data-skip-button])')) {
      const btn = createElementFromHTML(html`<button class="ui button">${el.id}</button`);
      btn.addEventListener('click', () => showFomanticModal(el));
      modalButtons.append(btn);
    }
  }

  const sampleButtons = document.querySelectorAll('#devtest-button-samples button.ui.button');
  if (sampleButtons.length) {
    const buttonStyles = document.querySelectorAll<HTMLInputElement>('input[name*="button-style"]');
    for (const elStyle of buttonStyles) {
      elStyle.addEventListener('click', () => {
        for (const btn of sampleButtons) {
          for (const el of buttonStyles) {
            if (el.value) btn.classList.toggle(el.value, el.checked);
          }
        }
      });
    }
    const buttonStates = document.querySelectorAll<HTMLInputElement>('input[name*="button-state"]');
    for (const elState of buttonStates) {
      elState.addEventListener('click', () => {
        for (const btn of sampleButtons) {
          (btn as any)[elState.value] = elState.checked;
        }
      });
    }
  }
}

export function initDevtest() {
  registerGlobalInitFunc('initDevtestPage', initDevtestPage);
  registerGlobalInitFunc('initDevtestDetailsErrorMessage', () => {
    for (let i = 0; i < 2; i++) {
      showGlobalErrorMessage('showGlobalErrorMessage single message', 'warning');
      showGlobalErrorMessage('showGlobalErrorMessage message with details', 'error', `detail message 1\nvery lo${'o'.repeat(200)}ng line 2\nline 3`);
    }
  });
}
