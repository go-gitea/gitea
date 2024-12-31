import {htmlEscape} from 'escape-goat';
import {svg} from '../svg.ts';
import {animateOnce, showElem} from '../utils/dom.ts';
import Toastify from 'toastify-js'; // don't use "async import", because when network error occurs, the "async import" also fails and nothing is shown
import type {Intent} from '../types.ts';
import type {SvgName} from '../svg.ts';
import type {Options} from 'toastify-js';
import type StartToastifyInstance from 'toastify-js';

export type Toast = ReturnType<typeof StartToastifyInstance>;

type ToastLevels = {
  [intent in Intent]: {
    icon: SvgName,
    background: string,
    duration: number,
  }
}

const levels: ToastLevels = {
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

type ToastOpts = {
  useHtmlBody?: boolean,
  preventDuplicates?: boolean,
} & Options;

// See https://github.com/apvarun/toastify-js#api for options
function showToast(message: string, level: Intent, {gravity, position, duration, useHtmlBody, preventDuplicates = true, ...other}: ToastOpts = {}): Toast {
  const body = useHtmlBody ? String(message) : htmlEscape(message);
  const key = `${level}-${body}`;

  // prevent showing duplicate toasts with same level and message, and give a visual feedback for end users
  if (preventDuplicates) {
    const toastEl = document.querySelector(`.toastify[data-toast-unique-key="${CSS.escape(key)}"]`);
    if (toastEl) {
      const toastDupNumEl = toastEl.querySelector('.toast-duplicate-number');
      showElem(toastDupNumEl);
      toastDupNumEl.textContent = String(Number(toastDupNumEl.textContent) + 1);
      animateOnce(toastDupNumEl, 'pulse-1p5-200');
      return;
    }
  }

  const {icon, background, duration: levelDuration} = levels[level ?? 'info'];
  const toast = Toastify({
    text: `
      <div class='toast-icon'>${svg(icon)}</div>
      <div class='toast-body'><span class="toast-duplicate-number tw-hidden">1</span>${body}</div>
      <button class='btn toast-close'>${svg('octicon-x')}</button>
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
  return toast;
}

export function showInfoToast(message: string, opts?: ToastOpts): Toast {
  return showToast(message, 'info', opts);
}

export function showWarningToast(message: string, opts?: ToastOpts): Toast {
  return showToast(message, 'warning', opts);
}

export function showErrorToast(message: string, opts?: ToastOpts): Toast {
  return showToast(message, 'error', opts);
}
