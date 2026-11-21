import {svg} from '../svg.ts';
import {createElementFromAttrs, queryElems} from '../utils/dom.ts';
import {localUserSettings} from '../modules/user-settings.ts';

export function initMarkupCodeBlocks(elMarkup: HTMLElement): void {
  queryElems(elMarkup, '.code-block code:not(.language-math):not(.language-mermaid)', (el) => {
    if (!el.textContent) return;

    let container = el.closest('.code-block-container');
    if (!container) {
      const codeBlock = el.closest('.code-block')!;
      container = createElementFromAttrs('div', {class: 'code-block-container'});
      codeBlock.replaceWith(container);
      container.append(codeBlock);
    }
    const button = createElementFromAttrs<HTMLButtonElement>('button', {
      class: 'ui compact icon button code-wrap auto-hide-control',
      type: 'button',
      'data-tooltip-content': window.config.i18n.code_toggle_wrap,
      'aria-label': window.config.i18n.code_toggle_wrap,
    });
    button.innerHTML = svg('material-wrap-text');
    let wrap = localUserSettings.getBoolean('wrap-markup-code');
    const updateWrap = () => {
      container.classList.toggle('code-overflow-wrap', wrap);
      container.classList.toggle('code-overflow-scroll', !wrap);
      button.setAttribute('aria-pressed', String(wrap));
    };
    updateWrap();
    button.addEventListener('click', () => {
      wrap = !wrap;
      updateWrap();
      localUserSettings.setBoolean('wrap-markup-code', wrap);
    });
    container.append(button);
  });
}
