import {hideElem, showElem} from '../utils/dom.js';

export function initUnicodeEscapeButton() {
  document.addEventListener('click', (e) => {
    const target = e.target;
    const fileView = target.closest('.file-content, .non-diff-file-content')?.querySelectorAll('.file-code, .file-view');
    if (target.matches('.escape-button')) {
      e.preventDefault();
      for (const el of fileView) el.classList.add('unicode-escaped');
      hideElem(target);
      showElem('.unescape-button');
    } else if (target.matches('.unescape-button')) {
      e.preventDefault();
      for (const el of fileView) el.classList.remove('unicode-escaped');
      hideElem(target);
      showElem('.escape-button');
    } else if (target.matches('.toggle-escape-button')) {
      e.preventDefault();
      const isEscaped = fileView[0].classList.contains('unicode-escaped');
      for (const el of fileView) {
        if (isEscaped) {
          el.classList.remove('unicode-escaped');
        } else {
          el.classList.add('unicode-escaped');
        }
      }
      if (isEscaped) {
        hideElem('.unescape-button');
        showElem('.escape-button');
      } else {
        showElem('.unescape-button');
        hideElem('.escape-button');
      }
    }
  });
}
