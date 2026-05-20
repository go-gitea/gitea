import {clippie, type ClippieContent} from 'clippie';
import {toAbsoluteUrl} from '../utils.ts';
import {svg} from '../svg.ts';
import {createElementFromHTML} from '../utils/dom.ts';

const pendingFeedback = new WeakSet<HTMLElement>();

/** Copy `content` to the clipboard. If `target` is given, the first `<svg>` it contains shows
 *  success/fail feedback and repeated clicks on the same target are ignored. When `content` is
 *  a function, `target` also shows a spinner while it resolves. */
export async function copyToClipboard(content: ClippieContent | (() => Promise<ClippieContent>), target?: HTMLElement): Promise<boolean> {
  if (target && pendingFeedback.has(target)) return false;
  if (target) pendingFeedback.add(target);
  let success = false;
  try {
    if (typeof content === 'function') {
      if (target) {
        const svgEl = target.querySelector('svg');
        if (svgEl) target.style.setProperty('--loading-size', `${svgEl.getAttribute('width')!}px`);
        target.classList.add('is-loading', 'loading-icon-2px');
      }
      try {
        content = await content();
      } finally {
        if (target) {
          target.classList.remove('is-loading', 'loading-icon-2px');
          target.style.removeProperty('--loading-size');
        }
      }
    }
    if (content) success = await clippie(content);
  } catch (err) {
    console.error(err);
  }
  if (target) {
    if (content) {
      showCopyFeedback(target, success);
    } else {
      pendingFeedback.delete(target);
    }
  }
  return success;
}

function showCopyFeedback(target: HTMLElement, success: boolean) {
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
    const target = (e.target as HTMLElement).closest<HTMLElement>('[data-clipboard-text], [data-clipboard-target]');
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
      await copyToClipboard(text, target);
    }
  });
}
