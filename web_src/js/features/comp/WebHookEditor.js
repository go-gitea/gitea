const {csrfToken} = window.config;

export function initCompWebHookEditor() {
  if ($('.new.webhook').length === 0) {
    return;
  }

  $('.events.checkbox input').on('change', function () {
    if ($(this).is(':checked')) {
      $('.events.fields').show();
    }
  });
  $('.non-events.checkbox input').on('change', function () {
    if ($(this).is(':checked')) {
      $('.events.fields').hide();
    }
  });

  const updateContentType = function () {
    const visible = $('#http_method').val() === 'POST';
    $('#content_type').parent().parent()[visible ? 'show' : 'hide']();
  };
  updateContentType();
  $('#http_method').on('change', () => {
    updateContentType();
  });

  // Test delivery
  $('#test-delivery').on('click', function () {
    const $this = $(this);
    $this.addClass('loading disabled');
    $.post($this.data('link'), {
      _csrf: csrfToken
    }).done(
      setTimeout(() => {
        window.location.href = $this.data('redirect');
      }, 5000)
    );
  });
}
