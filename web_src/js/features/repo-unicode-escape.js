import $ from 'jquery';

export function initUnicodeEscapeButton() {
  $(document).on('click', 'a.escape-button', (e) => {
    e.preventDefault();
    $(e.target).parents('.file-content, .non-diff-file-content').find('.file-code, .file-view').addClass('unicode-escaped');
    $(e.target).hide();
    $(e.target).siblings('a.unescape-button').show();
  });
  $(document).on('click', 'a.unescape-button', (e) => {
    e.preventDefault();
    $(e.target).parents('.file-content, .non-diff-file-content').find('.file-code, .file-view').removeClass('unicode-escaped');
    $(e.target).hide();
    $(e.target).siblings('a.escape-button').show();
  });
  $(document).on('click', 'a.toggle-escape-button', (e) => {
    e.preventDefault();
    const fileContent = $(e.target).parents('.file-content, .non-diff-file-content');
    if (fileContent.length) {
      const fileView = fileContent.find('.file-code, .file-view');
      if (fileView.hasClass('unicode-escaped')) {
        fileView.removeClass('unicode-escaped');
        fileContent.find('a.unescape-button').hide();
        fileContent.find('a.escape-button').show();
        return;
      }
      fileView.addClass('unicode-escaped');
      fileContent.find('a.unescape-button').show();
      fileContent.find('a.escape-button').hide();
      return;
    }

    const unicodeEscaped = $(e.target).parents('.unicode-escaped');
    if (unicodeEscaped.length) {
      unicodeEscaped.removeClass('unicode-escaped');
      unicodeEscaped.addClass('not-unicode-escaped');
      return;
    }

    const notUnicodeEscaped = $(e.target).parents('.not-unicode-escaped');
    if (notUnicodeEscaped.length) {
      notUnicodeEscaped.addClass('unicode-escaped');
      notUnicodeEscaped.removeClass('not-unicode-escaped');
    }
  });
}
