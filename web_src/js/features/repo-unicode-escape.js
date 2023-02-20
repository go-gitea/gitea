import $ from 'jquery';
import {hideElem, showElem} from '../utils/dom.js';

export function initUnicodeEscapeButton() {
  $(document).on('click', 'a.escape-button', (e) => {
    e.preventDefault();
    $(e.target).parents('.file-content, .non-diff-file-content').find('.file-code, .file-view').addClass('unicode-escaped');
    hideElem($(e.target));
    showElem($(e.target).siblings('a.unescape-button'));
  });
  $(document).on('click', 'a.unescape-button', (e) => {
    e.preventDefault();
    $(e.target).parents('.file-content, .non-diff-file-content').find('.file-code, .file-view').removeClass('unicode-escaped');
    hideElem($(e.target));
    showElem($(e.target).siblings('a.escape-button'));
  });
  $(document).on('click', 'a.toggle-escape-button', (e) => {
    e.preventDefault();
    const fileContent = $(e.target).parents('.file-content, .non-diff-file-content');
    const fileView = fileContent.find('.file-code, .file-view');
    if (fileView.hasClass('unicode-escaped')) {
      fileView.removeClass('unicode-escaped');
      hideElem(fileContent.find('a.unescape-button'));
      showElem(fileContent.find('a.escape-button'));
    } else {
      fileView.addClass('unicode-escaped');
      showElem(fileContent.find('a.unescape-button'));
      hideElem(fileContent.find('a.escape-button'));
    }
  });
}
