import $ from 'jquery';
import {hideElem, showElem} from '../utils/dom.js';

export function initUnicodeEscapeButton() {
  $(document).on('click', '.escape-button', (e) => {
    e.preventDefault();
    $(e.target).parents('.file-content, .non-diff-file-content').find('.file-code, .file-view').addClass('unicode-escaped');
    hideElem($(e.target));
    showElem($(e.target).siblings('.unescape-button'));
  });
  $(document).on('click', '.unescape-button', (e) => {
    e.preventDefault();
    $(e.target).parents('.file-content, .non-diff-file-content').find('.file-code, .file-view').removeClass('unicode-escaped');
    hideElem($(e.target));
    showElem($(e.target).siblings('.escape-button'));
  });
  $(document).on('click', '.toggle-escape-button', (e) => {
    e.preventDefault();
    const fileContent = $(e.target).parents('.file-content, .non-diff-file-content');
    const fileView = fileContent.find('.file-code, .file-view');
    if (fileView.hasClass('unicode-escaped')) {
      fileView.removeClass('unicode-escaped');
      hideElem(fileContent.find('.unescape-button'));
      showElem(fileContent.find('.escape-button'));
    } else {
      fileView.addClass('unicode-escaped');
      showElem(fileContent.find('.unescape-button'));
      hideElem(fileContent.find('.escape-button'));
    }
  });
}
