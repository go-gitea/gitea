import {addDelegatedEventListener, hideElem, queryElemSiblings, showElem, toggleElem, toggleElemClass} from '../utils/dom.ts';

export function initUnicodeEscapeButton() {
  // buttons might appear on these pages: file view (code, rendered markdown), diff (commit, pr conversation, pr diff), blame, wiki
  addDelegatedEventListener(document, 'click', '.escape-button, .unescape-button, .toggle-escape-button', (btn, e) => {
    e.preventDefault();

    const unicodeContentSelector = btn.getAttribute('data-unicode-content-selector');
    const container = unicodeContentSelector ?
      document.querySelector(unicodeContentSelector)! :
      btn.closest('.file-content, .non-diff-file-content')!;
    const fileViews = container.querySelectorAll('.file-body, .file-view'); // a diff's source and rendered views share the state
    const targets = fileViews.length ? fileViews : container;
    if (btn.matches('.escape-button')) {
      toggleElemClass(targets, 'unicode-escaped', true);
      hideElem(btn);
      showElem(queryElemSiblings(btn, '.unescape-button'));
    } else if (btn.matches('.unescape-button')) {
      toggleElemClass(targets, 'unicode-escaped', false);
      hideElem(btn);
      showElem(queryElemSiblings(btn, '.escape-button'));
    } else if (btn.matches('.toggle-escape-button')) {
      const isEscaped = (fileViews[0] ?? container).classList.contains('unicode-escaped');
      toggleElemClass(targets, 'unicode-escaped', !isEscaped);
      toggleElem(container.querySelectorAll('.unescape-button'), !isEscaped);
      toggleElem(container.querySelectorAll('.escape-button'), isEscaped);
    }
  });
}
