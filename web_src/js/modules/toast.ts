import {html, htmlEscape, htmlRaw} from '../utils/html.ts';
import {svgRaw} from '../svg.ts';
import {animateOnce, queryElems, showElem} from '../utils/dom.ts';
import Toastify from 'toastify-js'; // don't use "async import", because when network error occurs, the "async import" also fails and nothing is shown
import type {Intent} from '../types.ts';
import type {SvgName} from '../svg.ts';
import type {Options} from 'toastify-js';
import type StartToastifyInstance from 'toastify-js';

export type Toast = ReturnType<typeof StartToastifyInstance>;

type ToastLevels = {
  [intent in Intent]: {
    icon: SvgName,
    duration: number,
  }
};

const levels: ToastLevels = {
  success: {
    icon: 'octicon-check',
    duration: 2500,
  },
  info: {
    icon: 'octicon-info',
    duration: 5000,
  },
  warning: {
    icon: 'gitea-exclamation',
    duration: -1, // requires dismissal to hide
  },
  error: {
    icon: 'gitea-exclamation',
    duration: -1, // requires dismissal to hide
  },
};

type ToastOpts = {
  useHtmlBody?: boolean,
  preventDuplicates?: boolean | string,
} & Options;

type ToastifyElement = HTMLElement & {_giteaToastifyInstance?: Toast};

/** See https://github.com/apvarun/toastify-js#api for options */
function showToast(message: string, level: Intent = 'info', {gravity, position, duration, useHtmlBody, preventDuplicates = true, ...other}: ToastOpts = {}): Toast | null {
  const parent = document.querySelector('.ui.dimmer.active') ?? document.body;
  const duplicateKey = preventDuplicates ? (typeof preventDuplicates === 'string' ? preventDuplicates : `${level}-${message}`) : '';

  // prevent showing duplicate toasts with the same level and message, and give visual feedback for end users
  if (preventDuplicates) {
    const toastEl = parent.querySelector(`:scope > .toastify.on[data-toast-unique-key="${CSS.escape(duplicateKey)}"]`);
    if (toastEl) {
      const toastDupNumEl = toastEl.querySelector('.toast-duplicate-number')!;
      showElem(toastDupNumEl);
      toastDupNumEl.textContent = String(Number(toastDupNumEl.textContent) + 1);
      animateOnce(toastDupNumEl, 'pulse-1p5-200');
      return null;
    }
  }

  const {icon, duration: levelDuration} = levels[level];
  const bodyHtml = useHtmlBody ? message : htmlEscape(message);
  const toast = Toastify({
    selector: parent,
    text: html`
      <div class='toast-icon'>${svgRaw(icon)}</div>
      <div class='toast-body'><span class="toast-duplicate-number tw-hidden">1</span>${htmlRaw(bodyHtml)}</div>
      <button class='btn toast-close'>${svgRaw('octicon-x')}</button>
    `,
    escapeMarkup: false,
    className: `toast-${level}`,
    gravity: gravity ?? 'top',
    position: position ?? 'center',
    duration: duration ?? levelDuration,
    ...other,
  });

  toast.showToast();
  const el = toast.toastElement as ToastifyElement;
  el.querySelector('.toast-close')!.addEventListener('click', () => toast.hideToast());
  el.setAttribute('data-toast-unique-key', duplicateKey);
  el._giteaToastifyInstance = toast;
  return toast;
}

export function showSuccessToast(message: string, opts?: ToastOpts): Toast | null {
  return showToast(message, 'success', opts);
}

export function showInfoToast(message: string, opts?: ToastOpts): Toast | null {
  return showToast(message, 'info', opts);
}

export function showWarningToast(message: string, opts?: ToastOpts): Toast | null {
  return showToast(message, 'warning', opts);
}

export function showErrorToast(message: string, opts?: ToastOpts): Toast | null {
  return showToast(message, 'error', opts);
}

function hideToastByElement(el: Element): void {
  (el as ToastifyElement)?._giteaToastifyInstance?.hideToast();
}

export function hideToastsFrom(parent: Element): void {
  queryElems(parent, ':scope > .toastify.on', hideToastByElement);
}

export function hideToastsAll(): void {
  queryElems(document, '.toastify.on', hideToastByElement);
}
