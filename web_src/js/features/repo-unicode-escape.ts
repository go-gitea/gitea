import {hideElem, queryElemSiblings, showElem, toggleElem} from '../utils/dom.ts';

export function initUnicodeEscapeButton() {
  document.addEventListener('click', (e) => {
    const btn = e.target.closest('.escape-button, .unescape-button, .toggle-escape-button');
    if (!btn) return;

    e.preventDefault();

    let fileContent = btn.closest('.file-content, .non-diff-file-content');
    if (!fileContent) {
      const tippyTarget = btn.closest('.tippy-target');
      if (tippyTarget) {
        fileContent = document.querySelectorAll(`[data-diff-file-menu-tippy-target-id="${tippyTarget.getAttribute('data-diff-file-menu-tippy-id')}"]`)[0].closest('.file-content, .non-diff-file-content');
      }
    }
    const fileView = fileContent?.querySelectorAll('.file-code, .file-view');
    if (!fileView) return;
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
