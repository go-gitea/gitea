import {clippie, type ClippieContent} from 'clippie';
import {showTemporaryTooltip} from './tippy.ts';
import {sleep, toAbsoluteUrl} from '../utils.ts';
import {svg} from '../svg.ts';
import {createElementFromHTML} from '../utils/dom.ts';

const {copy_success, copy_error} = window.config.i18n;
const pendingFeedback = new WeakSet<HTMLElement>();

/** Copy `content` to the clipboard. If `target` is given, its `.octicon-copy` is swapped to show
 *  success/fail feedback (or a tooltip if it has none), and repeated clicks on the same target are
 *  ignored. When `content` is a function, `target` also shows a spinner while it resolves. */
export async function copyToClipboard(target: HTMLElement, content: ClippieContent | (() => Promise<ClippieContent>)) {
  if (pendingFeedback.has(target)) return;
  pendingFeedback.add(target);
  let success = false;
  try {
    if (typeof content === 'function') {
      const svgEl = target.querySelector<SVGElement>('.octicon-copy');
      if (svgEl) target.style.setProperty('--loading-size', `${svgEl.getAttribute('width')!}px`);
      target.classList.add('is-loading', 'loading-icon-2px');
      try {
        content = await content();
      } finally {
        target.classList.remove('is-loading', 'loading-icon-2px');
        target.style.removeProperty('--loading-size');
      }
    }
    success = await clippie(content);
  } catch (err) {
    console.error(err);
  }
  await showCopyFeedback(target, success);
  pendingFeedback.delete(target);
}

export async function copyTextToClipboard(content: string): Promise<boolean> {
  return await clippie(content);
}

async function showCopyFeedback(target: HTMLElement, success: boolean) {
  const origSvg = target.querySelector<SVGElement>('.octicon-copy');
  if (!origSvg) {
    // menu items have no copy icon, so show a tooltip on the menu button instead
    showTemporaryTooltip(target, success ? copy_success : copy_error);
    return;
  }
  const restore = replaceWithFeedbackSvg(origSvg, success);
  await sleep(1000);
  restore();
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

    if (text === null) return;

    if (target.getAttribute('data-clipboard-text-type') === 'url') {
      text = toAbsoluteUrl(text);
    }

    await copyToClipboard(target, text);
  });
}
