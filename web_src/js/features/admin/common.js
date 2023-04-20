import $ from 'jquery';
import {checkAppUrl} from '../common-global.js';
import {hideElem, showElem, toggleElem} from '../../utils/dom.js';

const {csrfToken} = window.config;

export function initAdminCommon() {
  if ($('.page-content.admin').length === 0) {
    return;
  }

  // check whether appUrl(ROOT_URL) is correct, if not, show an error message
  // only admin pages need this check because most templates are using relative URLs now
  checkAppUrl();

  // New user
  if ($('.admin.new.user').length > 0 || $('.admin.edit.user').length > 0) {
    $('#login_type').on('change', function () {
      if ($(this).val().substring(0, 1) === '0') {
        $('#user_name').removeAttr('disabled');
        $('#login_name').removeAttr('required');
        hideElem($('.non-local'));
        showElem($('.local'));
        $('#user_name').trigger('focus');

        if ($(this).data('password') === 'required') {
          $('#password').attr('required', 'required');
        }
      } else {
        if ($('.admin.edit.user').length > 0) {
          $('#user_name').attr('disabled', 'disabled');
        }
        $('#login_name').attr('required', 'required');
        showElem($('.non-local'));
        hideElem($('.local'));
        $('#login_name').trigger('focus');

        $('#password').removeAttr('required');
      }
    });
  }

  function onSecurityProtocolChange() {
    if ($('#security_protocol').val() > 0) {
      showElem($('.has-tls'));
    } else {
      hideElem($('.has-tls'));
    }
  }

  function onUsePagedSearchChange() {
    if ($('#use_paged_search').prop('checked')) {
      showElem('.search-page-size');
      $('.search-page-size').find('input').attr('required', 'required');
    } else {
      hideElem('.search-page-size');
      $('.search-page-size').find('input').removeAttr('required');
    }
  }

  function onOAuth2Change(applyDefaultValues) {
    hideElem($('.open_id_connect_auto_discovery_url, .oauth2_use_custom_url'));
    $('.open_id_connect_auto_discovery_url input[required]').removeAttr('required');

    const provider = $('#oauth2_provider').val();
    switch (provider) {
      case 'openidConnect':
        $('.open_id_connect_auto_discovery_url input').attr('required', 'required');
        showElem($('.open_id_connect_auto_discovery_url'));
        break;
      default:
        if ($(`#${provider}_customURLSettings`).data('required')) {
          $('#oauth2_use_custom_url').attr('checked', 'checked');
        }
        if ($(`#${provider}_customURLSettings`).data('available')) {
          showElem($('.oauth2_use_custom_url'));
        }
    }
    onOAuth2UseCustomURLChange(applyDefaultValues);
  }

  function onOAuth2UseCustomURLChange(applyDefaultValues) {
    const provider = $('#oauth2_provider').val();
    hideElem($('.oauth2_use_custom_url_field'));
    $('.oauth2_use_custom_url_field input[required]').removeAttr('required');

    if ($('#oauth2_use_custom_url').is(':checked')) {
      for (const custom of ['token_url', 'auth_url', 'profile_url', 'email_url', 'tenant']) {
        if (applyDefaultValues) {
          $(`#oauth2_${custom}`).val($(`#${provider}_${custom}`).val());
        }
        if ($(`#${provider}_${custom}`).data('available')) {
          $(`.oauth2_${custom} input`).attr('required', 'required');
          showElem($(`.oauth2_${custom}`));
        }
      }
    }
  }

  function onEnableLdapGroupsChange() {
    toggleElem($('#ldap-group-options'), $('.js-ldap-group-toggle').is(':checked'));
  }

  // New authentication
  if ($('.admin.new.authentication').length > 0) {
    $('#auth_type').on('change', function () {
      hideElem($('.ldap, .dldap, .smtp, .pam, .oauth2, .has-tls, .search-page-size, .sspi'));

      $('.ldap input[required], .binddnrequired input[required], .dldap input[required], .smtp input[required], .pam input[required], .oauth2 input[required], .has-tls input[required], .sspi input[required]').removeAttr('required');
      $('.binddnrequired').removeClass('required');

      const authType = $(this).val();
      switch (authType) {
        case '2': // LDAP
          showElem($('.ldap'));
          $('.binddnrequired input, .ldap div.required:not(.dldap) input').attr('required', 'required');
          $('.binddnrequired').addClass('required');
          break;
        case '3': // SMTP
          showElem($('.smtp'));
          showElem($('.has-tls'));
          $('.smtp div.required input, .has-tls').attr('required', 'required');
          break;
        case '4': // PAM
          showElem($('.pam'));
          $('.pam input').attr('required', 'required');
          break;
        case '5': // LDAP
          showElem($('.dldap'));
          $('.dldap div.required:not(.ldap) input').attr('required', 'required');
          break;
        case '6': // OAuth2
          showElem($('.oauth2'));
          $('.oauth2 div.required:not(.oauth2_use_custom_url,.oauth2_use_custom_url_field,.open_id_connect_auto_discovery_url) input').attr('required', 'required');
          onOAuth2Change(true);
          break;
        case '7': // SSPI
          showElem($('.sspi'));
          $('.sspi div.required input').attr('required', 'required');
          break;
      }
      if (authType === '2' || authType === '5') {
        onSecurityProtocolChange();
        onEnableLdapGroupsChange();
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
    $('.js-ldap-group-toggle').on('change', onEnableLdapGroupsChange);
  }
  // Edit authentication
  if ($('.admin.edit.authentication').length > 0) {
    const authType = $('#auth_type').val();
    if (authType === '2' || authType === '5') {
      $('#security_protocol').on('change', onSecurityProtocolChange);
      $('.js-ldap-group-toggle').on('change', onEnableLdapGroupsChange);
      onEnableLdapGroupsChange();
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
      $detailModal.find('.sub.header').text($(this).parents('tr').find('relative-time').attr('title'));
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
    $('#delete-selection').on('click', function (e) {
      e.preventDefault();
      const $this = $(this);
      $this.addClass('loading disabled');
      const ids = [];
      $checkboxes.each(function () {
        if ($(this).checkbox('is checked')) {
          ids.push($(this).data('id'));
        }
      });
      $.post($this.data('link'), {
        _csrf: csrfToken,
        ids
      }).done(() => {
        window.location.href = $this.data('redirect');
      });
    });
  }
}
