import {clippie, type ClippieContent} from 'clippie';
import {showTemporaryTooltip} from './tippy.ts';
import {sleep} from '../utils.ts';
import {svg} from '../svg.ts';
import {createElementFromHTML} from '../utils/dom.ts';

const {copy_success, copy_error} = window.config.i18n;
const pendingFeedback = new WeakSet<HTMLElement>();

/** copy the copiable content to clipboard, return "true" on success, otherwise "false" */
export async function copyToClipboard(content: ClippieContent): Promise<boolean> {
  return await clippie(content);
}

/** Copy `content` to the clipboard. `target` is used to:
 *  - avoid duplicate copy actions (especially when the content will be fetched from an async function)
 *  - provide feedback to end users (its `.octicon-copy` is swapped to show success/fail feedback, or a tooltip if it has none)
 *  When `content` is a function, `target` also shows a spinner while it resolves. */
export async function copyToClipboardWithFeedback(target: HTMLElement, content: ClippieContent | (() => Promise<ClippieContent>)) {
  if (pendingFeedback.has(target)) return;
  pendingFeedback.add(target);

  let success = false;
  const feedbackSvg = target.querySelector<SVGElement>('.octicon-copy');

  // prepare copiable "content"
  try {
    if (typeof content === 'function') {
      if (feedbackSvg) target.style.setProperty('--loading-size', `${feedbackSvg.getAttribute('width')!}px`);
      target.classList.add('is-loading', 'loading-icon-2px');
      try {
        content = await content();
      } finally {
        target.classList.remove('is-loading', 'loading-icon-2px');
        target.style.removeProperty('--loading-size');
      }
    }
    success = await copyToClipboard(content);
  } catch (err) {
    console.error(err);
  }

  // show feedback
  if (feedbackSvg) {
    const restore = replaceWithFeedbackSvg(feedbackSvg, success);
    await sleep(1000);
    restore();
  } else {
    showTemporaryTooltip(target, success ? copy_success : copy_error);
  }

  pendingFeedback.delete(target);
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
    // now, text can not be null
    await copyToClipboardWithFeedback(target, text);
  });
}
