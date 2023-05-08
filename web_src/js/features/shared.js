import $ from 'jquery';

const {csrfToken} = window.config;

export function initSettingVariables() {
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

    const url = $(this).attr('data-base-action');
    const commitButton = $modal.find('.actions > .ok.button');
    $(commitButton).on('click', (e) => {
      e.preventDefault();
      $.post(url, {
        _csrf: csrfToken,
        name: $modal.find('input[name=name]').val(),
        data: $modal.find('textarea[name=data]').val(),
      }, (data) => {
        if (data.redirect) window.location.href = data.redirect;
        else window.location.reload();
      });
    });
  });
}