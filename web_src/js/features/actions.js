import $ from 'jquery';

const {csrfToken} = window.config;

export function initActionsVariables() {
  $('.show-variable-edit-modal').on('click', function () {
    const target = $(this).attr('data-modal');
    const $modal = $(target);
    // clear input/textarea value
    $modal.find('input[name=name]').val('');
    $modal.find('textarea[name=data]').val('');
    // set dialog header
    const $header = $modal.find('#variable-edit-header');
    $header.text($(this).attr('data-modal-header'));

    if ($(this).attr('data-is-new') === 'false') {
      // edit variable dialog
      const oldName = $(this).attr('data-old-name');
      const oldValue = $(this).attr('data-old-value');
      $modal.find('input[name=name]').val(oldName);
      $modal.find('textarea[name=data]').val(oldValue);
    }

    const commitButton = $modal.find('.actions > .ok.button');
    $(commitButton).on('click', async (e) => {
      e.preventDefault();
      const url = $(this).attr('data-base-action');
      const res = await fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Csrf-Token': csrfToken,
        },
        body: JSON.stringify({
          name: $modal.find('input[name=name]').val(),
          data: $modal.find('textarea[name=data]').val(),
        }),
      });
      const data = await res.json();
      if (data.redirect) window.location.href = data.redirect;
      else window.location.reload();
    });
  });
}
