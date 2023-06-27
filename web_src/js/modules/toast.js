import {htmlEscape} from 'escape-goat';
import {svg} from '../svg.js';

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
async function showToast(message, level, {gravity, position, duration, ...other} = {}) {
  if (!message) return;

  const {default: Toastify} = await import(/* webpackChunkName: 'toastify' */'toastify-js');
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

  toast.toastElement.querySelector('.toast-close').addEventListener('click', () => {
    toast.removeElement(toast.toastElement);
  });
}

export async function showInfoToast(message, opts) {
  return await showToast(message, 'info', opts);
}

export async function showWarningToast(message, opts) {
  return await showToast(message, 'warning', opts);
}

export async function showErrorToast(message, opts) {
  return await showToast(message, 'error', opts);
}
