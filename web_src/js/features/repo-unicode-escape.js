export function initEscapeButton() {
  $('a.escape-button').on('click', (e) => {
    e.preventDefault();
    $(e.target).parents('.file-content, .non-diff-file-content').find('.file-code, .file-view').addClass('unicode-escaped');
    $(e.target).hide();
    $(e.target).siblings('a.unescape-button').show();
  });
  $('a.unescape-button').on('click', (e) => {
    e.preventDefault();
    $(e.target).parents('.file-content, .non-diff-file-content').find('.file-code, .file-view').removeClass('unicode-escaped');
    $(e.target).hide();
    $(e.target).siblings('a.escape-button').show();
  });
}
