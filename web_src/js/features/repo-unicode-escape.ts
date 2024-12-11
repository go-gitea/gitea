import {addDelegatedEventListener, hideElem, queryElemSiblings, showElem, toggleElem} from '../utils/dom.ts';

export function initUnicodeEscapeButton() {
  addDelegatedEventListener(document, 'click', '.escape-button, .unescape-button, .toggle-escape-button', (btn, e) => {
    e.preventDefault();

    const fileContentElemId = btn.getAttribute('data-file-content-elem-id');
    const fileContent = fileContentElemId ?
      document.querySelector(`#${fileContentElemId}`) :
      btn.closest('.file-content, .non-diff-file-content');
    const fileView = fileContent?.querySelectorAll('.file-code, .file-view');
    if (btn.matches('.escape-button')) {
      for (const el of fileView) el.classList.add('unicode-escaped');
      hideElem(btn);
      showElem(queryElemSiblings(btn, '.unescape-button'));
    } else if (btn.matches('.unescape-button')) {
      for (const el of fileView) el.classList.remove('unicode-escaped');
      hideElem(btn);
      showElem(queryElemSiblings(btn, '.escape-button'));
    } else if (btn.matches('.toggle-escape-button')) {
      const isEscaped = fileView[0]?.classList.contains('unicode-escaped');
      for (const el of fileView) el.classList.toggle('unicode-escaped', !isEscaped);
      toggleElem(fileContent.querySelectorAll('.unescape-button'), !isEscaped);
      toggleElem(fileContent.querySelectorAll('.escape-button'), isEscaped);
    }
  });
}
