const {csrf} = window.config;

export function initAdminCommon() {
  if ($('.admin').length === 0) {
    return;
  }

  // New user
  if ($('.admin.new.user').length > 0 || $('.admin.edit.user').length > 0) {
    $('#login_type').on('change', function () {
      if ($(this).val().substring(0, 1) === '0') {
        $('#user_name').removeAttr('disabled');
        $('#login_name').removeAttr('required');
        $('.non-local').hide();
        $('.local').show();
        $('#user_name').focus();

        if ($(this).data('password') === 'required') {
          $('#password').attr('required', 'required');
        }
      } else {
        if ($('.admin.edit.user').length > 0) {
          $('#user_name').attr('disabled', 'disabled');
        }
        $('#login_name').attr('required', 'required');
        $('.non-local').show();
        $('.local').hide();
        $('#login_name').focus();

        $('#password').removeAttr('required');
      }
    });
  }

  function onSecurityProtocolChange() {
    if ($('#security_protocol').val() > 0) {
      $('.has-tls').show();
    } else {
      $('.has-tls').hide();
    }
  }

  function onUsePagedSearchChange() {
    if ($('#use_paged_search').prop('checked')) {
      $('.search-page-size').show()
        .find('input').attr('required', 'required');
    } else {
      $('.search-page-size').hide()
        .find('input').removeAttr('required');
    }
  }

  function onOAuth2Change(applyDefaultValues) {
    $('.open_id_connect_auto_discovery_url, .oauth2_use_custom_url').hide();
    $('.open_id_connect_auto_discovery_url input[required]').removeAttr('required');

    const provider = $('#oauth2_provider').val();
    switch (provider) {
      case 'openidConnect':
        $('.open_id_connect_auto_discovery_url input').attr('required', 'required');
        $('.open_id_connect_auto_discovery_url').show();
        break;
      default:
        if ($(`#${provider}_customURLSettings`).data('required')) {
          $('#oauth2_use_custom_url').attr('checked', 'checked');
        }
        if ($(`#${provider}_customURLSettings`).data('available')) {
          $('.oauth2_use_custom_url').show();
        }
    }
    onOAuth2UseCustomURLChange(applyDefaultValues);
  }

  function onOAuth2UseCustomURLChange(applyDefaultValues) {
    const provider = $('#oauth2_provider').val();
    $('.oauth2_use_custom_url_field').hide();
    $('.oauth2_use_custom_url_field input[required]').removeAttr('required');

    if ($('#oauth2_use_custom_url').is(':checked')) {
      for (const custom of ['token_url', 'auth_url', 'profile_url', 'email_url', 'tenant']) {
        if (applyDefaultValues) {
          $(`#oauth2_${custom}`).val($(`#${provider}_${custom}`).val());
        }
        if ($(`#${provider}_${custom}`).data('available')) {
          $(`.oauth2_${custom} input`).attr('required', 'required');
          $(`.oauth2_${custom}`).show();
        }
      }
    }
  }

  function onVerifyGroupMembershipChange() {
    if ($('#groups_enabled').is(':checked')) {
      $('#groups_enabled_change').show();
    } else {
      $('#groups_enabled_change').hide();
    }
  }

  // New authentication
  if ($('.admin.new.authentication').length > 0) {
    $('#auth_type').on('change', function () {
      $('.ldap, .dldap, .smtp, .pam, .oauth2, .has-tls, .search-page-size, .sspi').hide();

      $('.ldap input[required], .binddnrequired input[required], .dldap input[required], .smtp input[required], .pam input[required], .oauth2 input[required], .has-tls input[required], .sspi input[required]').removeAttr('required');
      $('.binddnrequired').removeClass('required');

      const authType = $(this).val();
      switch (authType) {
        case '2': // LDAP
          $('.ldap').show();
          $('.binddnrequired input, .ldap div.required:not(.dldap) input').attr('required', 'required');
          $('.binddnrequired').addClass('required');
          break;
        case '3': // SMTP
          $('.smtp').show();
          $('.has-tls').show();
          $('.smtp div.required input, .has-tls').attr('required', 'required');
          break;
        case '4': // PAM
          $('.pam').show();
          $('.pam input').attr('required', 'required');
          break;
        case '5': // LDAP
          $('.dldap').show();
          $('.dldap div.required:not(.ldap) input').attr('required', 'required');
          break;
        case '6': // OAuth2
          $('.oauth2').show();
          $('.oauth2 div.required:not(.oauth2_use_custom_url,.oauth2_use_custom_url_field,.open_id_connect_auto_discovery_url) input').attr('required', 'required');
          onOAuth2Change(true);
          break;
        case '7': // SSPI
          $('.sspi').show();
          $('.sspi div.required input').attr('required', 'required');
          break;
      }
      if (authType === '2' || authType === '5') {
        onSecurityProtocolChange();
        onVerifyGroupMembershipChange();
      }
      if (authType === '2') {
        onUsePagedSearchChange();
      }
    });
    $('#auth_type').trigger('change');
    $('#security_protocol').on('change', onSecurityProtocolChange);
    $('#use_paged_search').on('change', onUsePagedSearchChange);
    $('#oauth2_provider').on('change', () => onOAuth2Change(true));
    $('#oauth2_use_custom_url').on('change', () => onOAuth2UseCustomURLChange(true));
    $('#groups_enabled').on('change', onVerifyGroupMembershipChange);
  }
  // Edit authentication
  if ($('.admin.edit.authentication').length > 0) {
    const authType = $('#auth_type').val();
    if (authType === '2' || authType === '5') {
      $('#security_protocol').on('change', onSecurityProtocolChange);
      $('#groups_enabled').on('change', onVerifyGroupMembershipChange);
      onVerifyGroupMembershipChange();
      if (authType === '2') {
        $('#use_paged_search').on('change', onUsePagedSearchChange);
      }
    } else if (authType === '6') {
      $('#oauth2_provider').on('change', () => onOAuth2Change(true));
      $('#oauth2_use_custom_url').on('change', () => onOAuth2UseCustomURLChange(false));
      onOAuth2Change(false);
    }
  }

  // Notice
  if ($('.admin.notice')) {
    const $detailModal = $('#detail-modal');

    // Attach view detail modals
    $('.view-detail').on('click', function () {
      $detailModal.find('.content pre').text($(this).parents('tr').find('.notice-description').text());
      $detailModal.find('.sub.header').text($(this).parents('tr').find('.notice-created-time').text());
      $detailModal.modal('show');
      return false;
    });

    // Select actions
    const $checkboxes = $('.select.table .ui.checkbox');
    $('.select.action').on('click', function () {
      switch ($(this).data('action')) {
        case 'select-all':
          $checkboxes.checkbox('check');
          break;
        case 'deselect-all':
          $checkboxes.checkbox('uncheck');
          break;
        case 'inverse':
          $checkboxes.checkbox('toggle');
          break;
      }
    });
    $('#delete-selection').on('click', function () {
      const $this = $(this);
      $this.addClass('loading disabled');
      const ids = [];
      $checkboxes.each(function () {
        if ($(this).checkbox('is checked')) {
          ids.push($(this).data('id'));
        }
      });
      $.post($this.data('link'), {
        _csrf: csrf,
        ids
      }).done(() => {
        window.location.href = $this.data('redirect');
      });
    });
  }
}
