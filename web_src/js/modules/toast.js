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
function showToast(message, level, {gravity, position, duration, useHtmlBody, preventDuplicates = true, ...other} = {}) {
  const body = useHtmlBody ? String(message) : htmlEscape(message);
  const key = `${level}-${body}`;

  // prevent showing duplicate toasts with same level and message, hide all existing toasts with same key
  if (preventDuplicates) {
    const toastElements = document.querySelectorAll(`.toastify[data-toast-unique-key="${CSS.escape(key)}"]`);
    for (const el of toastElements) {
      el.remove(); // "hideToast" only removes the toast after an unchangeable delay, so we need to remove it immediately to make the "reposition" work with new toasts
      el._toastInst?.hideToast();
    }
  }

  const {icon, background, duration: levelDuration} = levels[level ?? 'info'];
  const toast = Toastify({
    text: `
      <div class='toast-icon'>${svg(icon)}</div>
      <div class='toast-body'>${body}</div>
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
  toast.toastElement.setAttribute('data-toast-unique-key', key);
  toast.toastElement._toastInst = toast;
  return toast;
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
