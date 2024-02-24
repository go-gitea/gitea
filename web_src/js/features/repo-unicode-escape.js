import {hideElem, showElem, toggleElem} from '../utils/dom.js';

export function initUnicodeEscapeButton() {
  document.addEventListener('click', (e) => {
    const target = e.target;
    if (!target.closest('.escape-button, .unescape-button, .toggle-escape-button')) return;
    e.preventDefault();
    const fileContent = target.closest('.file-content, .non-diff-file-content');
    const fileView = fileContent?.querySelectorAll('.file-code, .file-view');
    if (target.closest('.escape-button')) {
      for (const el of fileView) el.classList.add('unicode-escaped');
      hideElem(target);
      const siblings = Array.from(target.parentNode.children).filter((child) => child !== target);
      showElem(siblings.filter((s) => s.matches('.unescape-button')));
    } else if (target.closest('.unescape-button')) {
      for (const el of fileView) el.classList.remove('unicode-escaped');
      hideElem(target);
      const siblings = Array.from(target.parentNode.children).filter((child) => child !== target);
      showElem(siblings.filter((s) => s.matches('.escape-button')));
    } else if (target.closest('.toggle-escape-button')) {
      const isEscaped = fileView[0].classList.contains('unicode-escaped');
      for (const el of fileView) {
        el.classList.toggle('unicode-escaped', isEscaped);
      }
      toggleElem(fileContent.querySelectorAll('.unescape-button'), !isEscaped);
      toggleElem(fileContent.querySelectorAll('.escape-button'), isEscaped);
    }
  });
}
