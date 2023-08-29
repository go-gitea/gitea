import {htmlEscape} from 'escape-goat';
import {svg} from '../svg.js';
import Toastify from 'toastify-js'; // don't use "async import", because when network error occurs, the "async import" also fails and nothing is shown

const levels = {
  info: {
    icon: 'octicon-check',
    background: 'var(--color-green)',
    duration: 2500,
  },
  warning: {
    icon: 'gitea-exclamation',
    background: 'var(--color-orange)',
    duration: -1, // requires dismissal to hide
  },
  error: {
    icon: 'gitea-exclamation',
    background: 'var(--color-red)',
    duration: -1, // requires dismissal to hide
  },
};

// See https://github.com/apvarun/toastify-js#api for options
function showToast(message, level, {gravity, position, duration, ...other} = {}) {
  const {icon, background, duration: levelDuration} = levels[level ?? 'info'];

  const toast = Toastify({
    text: `
      <div class='toast-icon'>${svg(icon)}</div>
      <div class='toast-body'>${htmlEscape(message)}</div>
      <button class='toast-close'>${svg('octicon-x')}</button>
    `,
    escapeMarkup: false,
    gravity: gravity ?? 'top',
    position: position ?? 'center',
    duration: duration ?? levelDuration,
    style: {background},
    ...other,
  });

  toast.showToast();
  toast.toastElement.querySelector('.toast-close').addEventListener('click', () => toast.hideToast());
}

export function showInfoToast(message, opts) {
  return showToast(message, 'info', opts);
}

export function showWarningToast(message, opts) {
  return showToast(message, 'warning', opts);
}

export function showErrorToast(message, opts) {
  return showToast(message, 'error', opts);
}
