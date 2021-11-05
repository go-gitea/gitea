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

export function initShowBidi() {
  $('a.code-has-bidi').on('click', (e) => {
    e.preventDefault();

    $('a.code-has-bidi').each((_, target) => {
      const inner = $(target).siblings().closest('.code-inner');
      const escaped = inner.data('escaped');
      let original = inner.data('original');

      if (escaped) {
        inner.html(original);
        inner.data('escaped', '');
      } else {
        if (!original) {
          original = $(inner).html();
          inner.data('original', original);
        }

        inner.html(original.replaceAll(/([\u202A\u202B\u202C\u202D\u202E\u2066\u2067\u2068\u2069])/g, (match) => {
          const value = match.charCodeAt(0).toString(16);
          return `<span class="escaped-char">&amp;#${value};</span>`;
        }));
        inner.data('escaped', 'escaped');
      }
    });
  });
}
