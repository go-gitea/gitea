import {addDelegatedEventListener, hideElem, queryElemSiblings, showElem, toggleElem} from '../utils/dom.ts';

export function initUnicodeEscapeButton() {
  // buttons might appear on these pages: file view (code, rendered markdown), diff (commit, pr conversation, pr diff), blame, wiki
  addDelegatedEventListener(document, 'click', '.escape-button, .unescape-button, .toggle-escape-button', (btn, e) => {
    e.preventDefault();

    const unicodeContentSelector = btn.getAttribute('data-unicode-content-selector');
    const container = unicodeContentSelector ?
      document.querySelector(unicodeContentSelector) :
      btn.closest('.file-content, .non-diff-file-content');
    const fileView = container.querySelector('.file-code, .file-view') ?? container;
    if (btn.matches('.escape-button')) {
      fileView.classList.add('unicode-escaped');
      hideElem(btn);
      showElem(queryElemSiblings(btn, '.unescape-button'));
    } else if (btn.matches('.unescape-button')) {
      fileView.classList.remove('unicode-escaped');
      hideElem(btn);
      showElem(queryElemSiblings(btn, '.escape-button'));
    } else if (btn.matches('.toggle-escape-button')) {
      const isEscaped = fileView.classList.contains('unicode-escaped');
      fileView.classList.toggle('unicode-escaped', !isEscaped);
      toggleElem(container.querySelectorAll('.unescape-button'), !isEscaped);
      toggleElem(container.querySelectorAll('.escape-button'), isEscaped);
    }
  });
}
