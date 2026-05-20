import {toAbsoluteUrl} from '../utils.ts';
import {clippie, type ClippieCopyable} from 'clippie';
import {svg} from '../svg.ts';
import {createElementFromHTML} from '../utils/dom.ts';

const pendingFeedback = new WeakSet<Element>();

export async function copyToClipboard(target: Element, source: ClippieCopyable | (() => Promise<ClippieCopyable>)): Promise<boolean> {
  if (pendingFeedback.has(target)) return false;
  pendingFeedback.add(target);
  const el = target as HTMLElement;
  let success = false;
  try {
    let content: ClippieCopyable;
    if (typeof source === 'function') {
      const width = target.querySelector('svg')?.getAttribute('width') ?? '16';
      el.style.setProperty('--loading-size', `${width}px`);
      el.classList.add('is-loading', 'loading-icon-2px');
      try {
        content = await source();
      } finally {
        el.classList.remove('is-loading', 'loading-icon-2px');
        el.style.removeProperty('--loading-size');
      }
    } else {
      content = source;
    }
    success = Boolean(content) && await clippie(content);
  } catch (err) {
    console.error(err);
  }
  showCopyFeedback(target, success);
  return success;
}

function showCopyFeedback(target: Element, success: boolean) {
  const origSvg = target.querySelector('svg');
  if (!origSvg) {
    pendingFeedback.delete(target);
    return;
  }
  const restore = replaceWithFeedbackSvg(origSvg, success);
  setTimeout(() => {
    restore();
    pendingFeedback.delete(target);
  }, 1000);
}

function replaceWithFeedbackSvg(origSvg: SVGElement, success: boolean): () => void {
  const size = Number(origSvg.getAttribute('width')!);
  const {icon, color} = success ?
    {icon: 'octicon-check', color: 'tw-text-green'} as const :
    {icon: 'octicon-x', color: 'tw-text-red'} as const;
  const newSvg = createElementFromHTML<SVGElement>(svg(icon, size, color));
  origSvg.replaceWith(newSvg);
  return () => newSvg.replaceWith(origSvg);
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
