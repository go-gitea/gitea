import {showInfoToast, showWarningToast, showErrorToast} from '../modules/toast.ts';
import type {Toast} from '../modules/toast.ts';

type LevelMap = Record<string, (message: string) => Toast | null>;

export function initDevtest() {
  const els = document.querySelectorAll('.toast-test-button');
  if (!els.length) return;
  const levelMap: LevelMap = {info: showInfoToast, warning: showWarningToast, error: showErrorToast};
  for (const el of els) {
    el.addEventListener('click', () => {
      const level = el.getAttribute('data-toast-level')!;
      const message = el.getAttribute('data-toast-message')!;
      levelMap[level](message);
    });
  }
}
