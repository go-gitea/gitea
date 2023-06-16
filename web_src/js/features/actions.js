import $ from 'jquery';

const {csrfToken} = window.config;

export function initActionsEditBtns() {
  if (!$('.show-variable-edit-modal,.show-secret-add-modal').length) return;

  $('.show-variable-edit-modal,.show-secret-add-modal').on('click', function () {
    const $btn = $(this);
    const target = $btn.attr('data-modal');
    const $modal = $(target);
    const form = $modal.find('form')[0];
    // clear input/textarea value
    $modal.find('input[name=title]').val('');
    $modal.find('textarea[name=content]').val('');
    // set dialog header
    const $header = $modal.find('#actions-modal-header');
    $header.text($btn.attr('data-modal-header'));

    if ($btn.attr('data-is-new') === 'false') {
      // edit variable dialog
      $modal.find('input[name=title]').val($btn.attr('data-old-title'));
      $modal.find('textarea[name=content]').val($btn.attr('data-old-content'));
    }

    $modal.find('.actions .ok.button').off('click').on('click', () => {
      if (!form.reportValidity()) return false;

      (async () => {
        const url = $(this).attr('data-base-action');
        const res = await fetch(url, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-Csrf-Token': csrfToken,
          },
          body: JSON.stringify({
            title: $modal.find('input[name=title]').val(),
            content: $modal.find('textarea[name=content]').val(),
          }),
        });
        const data = await res.json();
        if (data.redirect) window.location.href = data.redirect;
        else window.location.reload();
      })();

      return false; // tell fomantic to do not close the modal
    });
  });
}

