import {svg, type SvgName} from '../svg.ts';
import {queryElems} from '../utils/dom.ts';

export function makeCodeBlockButton(className: string, name: SvgName): HTMLButtonElement {
  const button = document.createElement('button');
  button.classList.add(className, 'btn');
  button.innerHTML = svg(name, 14);
  return button;
}

const getWrap = () => (localStorage.getItem('wrap-markup-code') || 'false') === 'true';
const saveWrap = (value: boolean) => localStorage.setItem('wrap-markup-code', String(value));

function updateWrap(container: Element, wrap: boolean) {
  container.classList.remove(wrap ? 'code-overflow-scroll' : 'code-overflow-wrap');
  container.classList.add(wrap ? 'code-overflow-wrap' : 'code-overflow-scroll');
}

export function initMarkupCodeBlocks(elMarkup: HTMLElement): void {
  // .markup .code-block code
  queryElems(elMarkup, '.code-block code', (el) => {
    if (!el.textContent) return;

    const copyBtn = makeCodeBlockButton('code-copy', 'octicon-copy');
    copyBtn.setAttribute('data-tooltip-content', window.config.i18n.copy);
    // remove final trailing newline introduced during HTML rendering
    copyBtn.setAttribute('data-clipboard-text', el.textContent.replace(/\r?\n$/, ''));

    // we only want to use `.code-block-container` if it exists, no matter `.code-block` exists or not.
    const container = el.closest('.code-block-container') ?? el.closest('.code-block');

    const wrapBtn = makeCodeBlockButton('code-wrap', 'material-wrap-text');
    const wrap = getWrap();
    wrapBtn.setAttribute('data-active', String(wrap));
    updateWrap(container, wrap);

    wrapBtn.setAttribute('data-tooltip-content', window.config.i18n.code_toggle_wrap);
    wrapBtn.addEventListener('click', (e) => {
      const wrap = !getWrap();
      updateWrap(container, wrap);
      (e.currentTarget as HTMLButtonElement).setAttribute('data-active', String(wrap));
      saveWrap(wrap);
    });

    const btnContainer = document.createElement('div');
    btnContainer.classList.add('code-block-buttons');
    btnContainer.append(wrapBtn);
    btnContainer.append(copyBtn);
    container.append(btnContainer);
  });
}
