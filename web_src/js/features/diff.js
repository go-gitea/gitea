export function initDiffShowMore() {
  $('#diff-files, #diff-file-boxes').on('click', '#diff-show-more-files, #diff-show-more-files-stats', (e) => {
    e.preventDefault();

    if ($(e.target).hasClass('disabled')) {
      return;
    }
    $('#diff-show-more-files, #diff-show-more-files-stats').addClass('disabled');

    const url = $('#diff-show-more-files, #diff-show-more-files-stats').data('href');
    $.ajax({
      type: 'GET',
      url,
    }).done((resp) => {
      if (!resp || resp.html === '' || resp.empty) {
        $('#diff-show-more-files, #diff-show-more-files-stats').removeClass('disabled');
        return;
      }
      $('#diff-too-many-files-stats').remove();
      $('#diff-files').append($(resp).find('#diff-files li'));
      $('#diff-incomplete').replaceWith($(resp).find('#diff-file-boxes').children());
    });
  });
}
