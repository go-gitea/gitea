import $ from 'jquery';
const {csrfToken} = window.config;

const initAuthenticationHeaderSection = function() {
  const $authHeaderSection = $('.auth-headers');

  if ($authHeaderSection.length === 0) {
    return;
  }

  const $checkbox = $authHeaderSection.find('.checkbox input');

  const updateHeaderContentType = function() {
    const isBasicAuth = $authHeaderSection.find('#auth_header_type').val() === 'basic';
    const $basicAuthFields = $authHeaderSection.find('.basic-auth');
    const $tokenAuthFields = $authHeaderSection.find('.token-auth');

    if (isBasicAuth) {
      $basicAuthFields.addClass("required");
      $basicAuthFields.find('input').attr("required", "");
      $basicAuthFields.show();

      $tokenAuthFields.removeClass("required");
      $tokenAuthFields.find('input').removeAttr("required");
      $tokenAuthFields.hide();
    } else {
      $basicAuthFields.removeClass("required");
      $basicAuthFields.find('input').removeAttr("required");
      $basicAuthFields.hide();

      $tokenAuthFields.addClass("required");
      $tokenAuthFields.find('input').attr("required", "");
      $tokenAuthFields.show();
    }
  };

  const updateHeaderCheckbox = function() {
    if ($checkbox.is(':checked')) {
      const $headerName = $authHeaderSection.find('#auth_header_name');
      $headerName.attr("required", "");
      $headerName.parent().addClass("required");
      $headerName.parent().show();
      $authHeaderSection.find('#auth_header_type').parent().parent().show();
      updateHeaderContentType();
    } else {
      $authHeaderSection.find('.auth-header').hide();
      $authHeaderSection.find('.auth-header').removeClass("required");
      $authHeaderSection.find('.auth-header input').removeAttr("required");
    }
  };

  if ($checkbox.is(':checked')) {
    updateHeaderCheckbox();
  }

  $checkbox.on('change', updateHeaderCheckbox);
  $authHeaderSection.find('#auth_header_type').on('change', updateHeaderContentType);
};

export function initCompWebHookEditor() {
  if ($('.new.webhook').length === 0) {
    return;
  }

  initAuthenticationHeaderSection();

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
