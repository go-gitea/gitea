import $ from 'jquery';
import {checkAppUrl} from '../common-global.js';
import {hideElem, showElem, toggleElem} from '../../utils/dom.js';
import {POST} from '../../modules/fetch.js';

const {appSubUrl} = window.config;

function onSecurityProtocolChange() {
  if (Number(document.getElementById('security_protocol')?.value) > 0) {
    showElem('.has-tls');
  } else {
    hideElem('.has-tls');
  }
}

export function initAdminCommon() {
  if (!document.querySelector('.page-content.admin')) return;

  // check whether appUrl(ROOT_URL) is correct, if not, show an error message
  checkAppUrl();

  // New user
  if ($('.admin.new.user').length > 0 || $('.admin.edit.user').length > 0) {
    document.getElementById('login_type')?.addEventListener('change', function () {
      if (this.value?.substring(0, 1) === '0') {
        document.getElementById('user_name')?.removeAttribute('disabled');
        document.getElementById('login_name')?.removeAttribute('required');
        hideElem('.non-local');
        showElem('.local');
        document.getElementById('user_name')?.focus();

        if (this.getAttribute('data-password') === 'required') {
          document.getElementById('password')?.setAttribute('required', 'required');
        }
      } else {
        if (document.querySelector('.admin.edit.user')) {
          document.getElementById('user_name')?.setAttribute('disabled', 'disabled');
        }
        document.getElementById('login_name')?.setAttribute('required', 'required');
        showElem('.non-local');
        hideElem('.local');
        document.getElementById('login_name')?.focus();

        document.getElementById('password')?.removeAttribute('required');
      }
    });
  }

  function onUsePagedSearchChange() {
    const searchPageSizeElements = document.querySelectorAll('.search-page-size');
    if (document.getElementById('use_paged_search').checked) {
      showElem('.search-page-size');
      for (const el of searchPageSizeElements) {
        el.querySelector('input')?.setAttribute('required', 'required');
      }
    } else {
      hideElem('.search-page-size');
      for (const el of searchPageSizeElements) {
        el.querySelector('input')?.removeAttribute('required');
      }
    }
  }

  function onOAuth2Change(applyDefaultValues) {
    hideElem('.open_id_connect_auto_discovery_url, .oauth2_use_custom_url');
    for (const input of document.querySelectorAll('.open_id_connect_auto_discovery_url input[required]')) {
      input.removeAttribute('required');
    }

    const provider = document.getElementById('oauth2_provider')?.value;
    switch (provider) {
      case 'openidConnect':
        for (const input of document.querySelectorAll('.open_id_connect_auto_discovery_url input')) {
          input.setAttribute('required', 'required');
        }
        showElem('.open_id_connect_auto_discovery_url');
        break;
      default:
        if (document.getElementById(`#${provider}_customURLSettings`)?.getAttribute('data-required')) {
          document.getElementById('oauth2_use_custom_url')?.setAttribute('checked', 'checked');
        }
        if (document.getElementById(`#${provider}_customURLSettings`)?.getAttribute('data-available')) {
          showElem('.oauth2_use_custom_url');
        }
    }
    onOAuth2UseCustomURLChange(applyDefaultValues);
  }

  function onOAuth2UseCustomURLChange(applyDefaultValues) {
    const provider = document.getElementById('oauth2_provider')?.value;
    hideElem('.oauth2_use_custom_url_field');
    for (const input of document.querySelectorAll('.oauth2_use_custom_url_field input[required]')) {
      input.removeAttribute('required');
    }

    if (document.getElementById('oauth2_use_custom_url')?.checked) {
      for (const custom of ['token_url', 'auth_url', 'profile_url', 'email_url', 'tenant']) {
        if (applyDefaultValues) {
          document.getElementById(`oauth2_${custom}`).value = document.getElementById(`${provider}_${custom}`).value;
        }
        const customInput = document.getElementById(`${provider}_${custom}`);
        if (customInput && customInput.getAttribute('data-available')) {
          for (const input of document.querySelectorAll(`.oauth2_${custom} input`)) {
            input.setAttribute('required', 'required');
          }
          showElem(`.oauth2_${custom}`);
        }
      }
    }
  }

  function onEnableLdapGroupsChange() {
    toggleElem(document.getElementById('ldap-group-options'), $('.js-ldap-group-toggle')[0].checked);
  }

  // New authentication
  if (document.querySelector('.admin.new.authentication')) {
    document.getElementById('auth_type')?.addEventListener('change', function () {
      hideElem('.ldap, .dldap, .smtp, .pam, .oauth2, .has-tls, .search-page-size, .sspi');

      for (const input of document.querySelectorAll('.ldap input[required], .binddnrequired input[required], .dldap input[required], .smtp input[required], .pam input[required], .oauth2 input[required], .has-tls input[required], .sspi input[required]')) {
        input.removeAttribute('required');
      }

      document.querySelector('.binddnrequired')?.classList.remove('required');

      const authType = this.value;
      switch (authType) {
        case '2': // LDAP
          showElem('.ldap');
          for (const input of document.querySelectorAll('.binddnrequired input, .ldap div.required:not(.dldap) input')) {
            input.setAttribute('required', 'required');
          }
          document.querySelector('.binddnrequired')?.classList.add('required');
          break;
        case '3': // SMTP
          showElem('.smtp');
          showElem('.has-tls');
          for (const input of document.querySelectorAll('.smtp div.required input, .has-tls')) {
            input.setAttribute('required', 'required');
          }
          break;
        case '4': // PAM
          showElem('.pam');
          for (const input of document.querySelectorAll('.pam input')) {
            input.setAttribute('required', 'required');
          }
          break;
        case '5': // LDAP
          showElem('.dldap');
          for (const input of document.querySelectorAll('.dldap div.required:not(.ldap) input')) {
            input.setAttribute('required', 'required');
          }
          break;
        case '6': // OAuth2
          showElem('.oauth2');
          for (const input of document.querySelectorAll('.oauth2 div.required:not(.oauth2_use_custom_url,.oauth2_use_custom_url_field,.open_id_connect_auto_discovery_url) input')) {
            input.setAttribute('required', 'required');
          }
          onOAuth2Change(true);
          break;
        case '7': // SSPI
          showElem('.sspi');
          for (const input of document.querySelectorAll('.sspi div.required input')) {
            input.setAttribute('required', 'required');
          }
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
    document.getElementById('security_protocol')?.addEventListener('change', onSecurityProtocolChange);
    document.getElementById('use_paged_search')?.addEventListener('change', onUsePagedSearchChange);
    document.getElementById('oauth2_provider')?.addEventListener('change', () => onOAuth2Change(true));
    document.getElementById('oauth2_use_custom_url')?.addEventListener('change', () => onOAuth2UseCustomURLChange(true));
    $('.js-ldap-group-toggle').on('change', onEnableLdapGroupsChange);
  }
  // Edit authentication
  if (document.querySelector('.admin.edit.authentication')) {
    const authType = document.getElementById('auth_type')?.value;
    if (authType === '2' || authType === '5') {
      document.getElementById('security_protocol')?.addEventListener('change', onSecurityProtocolChange);
      $('.js-ldap-group-toggle').on('change', onEnableLdapGroupsChange);
      onEnableLdapGroupsChange();
      if (authType === '2') {
        document.getElementById('use_paged_search')?.addEventListener('change', onUsePagedSearchChange);
      }
    } else if (authType === '6') {
      document.getElementById('oauth2_provider')?.addEventListener('change', () => onOAuth2Change(true));
      document.getElementById('oauth2_use_custom_url')?.addEventListener('change', () => onOAuth2UseCustomURLChange(false));
      onOAuth2Change(false);
    }
  }

  if (document.querySelector('.admin.authentication')) {
    $('#auth_name').on('input', function () {
      // appSubUrl is either empty or is a path that starts with `/` and doesn't have a trailing slash.
      document.getElementById('oauth2-callback-url').textContent = `${window.location.origin}${appSubUrl}/user/oauth2/${encodeURIComponent(this.value)}/callback`;
    }).trigger('input');
  }

  // Notice
  if (document.querySelector('.admin.notice')) {
    const $detailModal = document.getElementById('detail-modal');

    // Attach view detail modals
    $('.view-detail').on('click', function () {
      $detailModal.find('.content pre').text($(this).parents('tr').find('.notice-description').text());
      $detailModal.find('.sub.header').text(this.closest('tr')?.querySelector('relative-time')?.getAttribute('title'));
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
    document.getElementById('delete-selection')?.addEventListener('click', async function (e) {
      e.preventDefault();
      this.classList.add('is-loading', 'disabled');
      const data = new FormData();
      $checkboxes.each(function () {
        if ($(this).checkbox('is checked')) {
          data.append('ids[]', this.getAttribute('data-id'));
        }
      });
      await POST(this.getAttribute('data-link'), {data});
      window.location.href = this.getAttribute('data-redirect');
    });
  }
}
