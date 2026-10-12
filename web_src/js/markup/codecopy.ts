import {svg} from '../svg.ts';
import {createElementFromAttrs, queryElems} from '../utils/dom.ts';
import {copyToClipboardWithFeedback} from '../modules/clipboard.ts';

export function makeCodeCopyButton(attrs: Record<string, string> = {}): HTMLButtonElement {
  const btn = createElementFromAttrs<HTMLButtonElement>('button', {
    class: 'ui compact icon button code-copy auto-hide-control',
    type: 'button',
    ...attrs,
  });
  btn.innerHTML = svg('octicon-copy');
  return btn;
}

export function initMarkupCodeCopy(elMarkup: HTMLElement): void {
  queryElems(elMarkup, '.code-block code', (el) => {
    // the code block's content can change (e.g.: debian package page: select different distributions),
    const btn = makeCodeCopyButton();
    btn.addEventListener('click', async () => {
      let content = el.textContent;
      // standard: HTML DOM parser always use "\n", but if "textContent" is set by JS, then the line ending is preserved
      // so here we always normalize it to "\n"
      content = content.replace(/\r\n?/g, '\n');

      // if the content is single-line, we don't want to add a trailing newline, because it will be annoying when pasting into a shell
      // however, if the content is multiple-line, always add a trailing newline, it's more consistent and convenient to paste into a shell
      content = content.replace(/\n$/, '');
      if (content.includes('\n')) content += '\n';

      await copyToClipboardWithFeedback(btn, content);
    });
    // we only want to use `.code-block-container` if it exists, no matter `.code-block` exists or not.
    const btnContainer = el.closest('.code-block-container') ?? el.closest('.code-block')!;
    btnContainer.append(btn);
  });
}
