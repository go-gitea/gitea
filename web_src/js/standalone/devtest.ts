import {showInfoToast, showWarningToast, showErrorToast} from '../modules/toast.ts';

function initDevtestToast() {
  const levelMap = {info: showInfoToast, warning: showWarningToast, error: showErrorToast};
  for (const el of document.querySelectorAll('.toast-test-button')) {
    el.addEventListener('click', () => {
      const level = el.getAttribute('data-toast-level');
      const message = el.getAttribute('data-toast-message');
      levelMap[level](message);
    });
  }
}

initDevtestToast();
