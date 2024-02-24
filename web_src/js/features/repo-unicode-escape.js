import {hideElem, showElem} from '../utils/dom.js';

export function initUnicodeEscapeButton() {
  document.addEventListener('click', (e) => {
    const target = e.target;
    if (!target.matches('.escape-button, .unescape-button, .toggle-escape-button')) return;
    e.preventDefault();
    const fileContent = target.closest('.file-content, .non-diff-file-content');
    const fileView = fileContent?.querySelectorAll('.file-code, .file-view');
    if (target.matches('.escape-button')) {
      for (const el of fileView) el.classList.add('unicode-escaped');
      hideElem(target);
      const siblings = Array.from(target.parentNode.children).filter((child) => child !== target);
      showElem(siblings.filter((s) => s.matches('.unescape-button')));
    } else if (target.matches('.unescape-button')) {
      for (const el of fileView) el.classList.remove('unicode-escaped');
      hideElem(target);
      const siblings = Array.from(target.parentNode.children).filter((child) => child !== target);
      showElem(siblings.filter((s) => s.matches('.escape-button')));
    } else if (target.matches('.toggle-escape-button')) {
      const isEscaped = fileView[0].classList.contains('unicode-escaped');
      for (const el of fileView) {
        if (isEscaped) {
          el.classList.remove('unicode-escaped');
        } else {
          el.classList.add('unicode-escaped');
        }
      }
      if (isEscaped) {
        hideElem(fileContent.querySelectorAll('.unescape-button'));
        showElem(fileContent.querySelectorAll('.escape-button'));
      } else {
        showElem(fileContent.querySelectorAll('.unescape-button'));
        hideElem(fileContent.querySelectorAll('.escape-button'));
      }
    }
  });
}
