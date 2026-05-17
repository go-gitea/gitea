import {toAbsoluteUrl} from '../utils.ts';
import {clippie} from 'clippie';
import {svg} from '../svg.ts';
import {createElementFromHTML} from '../utils/dom.ts';

const pendingFeedback = new WeakSet<Element>();

type CopyContent = string | Blob;

export async function copyToClipboard(target: Element, content: CopyContent | (() => Promise<CopyContent>)): Promise<boolean> {
  if (pendingFeedback.has(target)) return false;
  pendingFeedback.add(target);
  let resolved: CopyContent = '';
  try {
    if (typeof content === 'function') {
      const width = target.querySelector('svg')?.getAttribute('width') ?? '16';
      (target as HTMLElement).style.setProperty('--loading-size', `${width}px`);
      target.classList.add('is-loading', 'loading-icon-2px');
      try {
        resolved = await content();
      } finally {
        target.classList.remove('is-loading', 'loading-icon-2px');
        (target as HTMLElement).style.removeProperty('--loading-size');
      }
    } else {
      resolved = content;
    }
  } catch (err) {
    console.error(err);
  }
  const success = Boolean(resolved) && await clippie(resolved);
  showCopyFeedback(target, success);
  return success;
}

function showCopyFeedback(target: Element, success: boolean) {
  const origSvg = target.querySelector('svg');
  if (!origSvg) {
    pendingFeedback.delete(target);
    return;
  }
  const newSvg = replaceWithFeedbackSvg(origSvg, success);
  setTimeout(() => {
    newSvg.replaceWith(origSvg);
    pendingFeedback.delete(target);
  }, 1000);
}

function replaceWithFeedbackSvg(origSvg: SVGElement, success: boolean): SVGElement {
  const size = Number(origSvg.getAttribute('width')!);
  const {icon, color} = success ?
    {icon: 'octicon-check', color: 'tw-text-green'} as const :
    {icon: 'octicon-x', color: 'tw-text-red'} as const;
  const newSvg = createElementFromHTML<SVGElement>(svg(icon, size, color));
  origSvg.replaceWith(newSvg);
  return newSvg;
}

// Enable clipboard copy from HTML attributes. These properties are supported:
// - data-clipboard-text: Direct text to copy
// - data-clipboard-target: Holds a selector for an element. "value" of <input> or <textarea>, or "textContent" of <div> will be copied
// - data-clipboard-text-type: When set to 'url' will convert relative to absolute urls
export function initGlobalCopyToClipboardListener() {
  document.addEventListener('click', async (e) => {
    const target = (e.target as HTMLElement).closest('[data-clipboard-text], [data-clipboard-target]');
    if (!target) return;

    e.preventDefault();

    let text = target.getAttribute('data-clipboard-text');
    if (text === null) {
      const textSelector = target.getAttribute('data-clipboard-target')!;
      const textTarget = document.querySelector(textSelector)!;
      if (textTarget.nodeName === 'INPUT' || textTarget.nodeName === 'TEXTAREA') {
        text = (textTarget as HTMLInputElement | HTMLTextAreaElement).value;
      } else if (textTarget.nodeName === 'DIV') {
        text = textTarget.textContent;
      } else {
        throw new Error(`Unsupported element for clipboard target: ${textSelector}`);
      }
    }

    if (text && target.getAttribute('data-clipboard-text-type') === 'url') {
      text = toAbsoluteUrl(text);
    }

    if (text) {
      await copyToClipboard(target, text);
    }
  });
}
