import $ from 'jquery';
import {checkAppUrl} from '../common-global.js';
import {hideElem, showElem, toggleElem} from '../../utils/dom.js';
import {POST} from '../../modules/fetch.js';

const {appSubUrl} = window.config;

function onSecurityProtocolChange() {
  if (Number(document.querySelector('#security_protocol')?.value) > 0) {
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
    document.querySelector('#login_type')?.addEventListener('change', function () {
      if (this.value?.substring(0, 1) === '0') {
        document.querySelector('#user_name')?.removeAttribute('disabled');
        document.querySelector('#login_name')?.removeAttribute('required');
        hideElem('.non-local');
        showElem('.local');
        document.querySelector('#user_name')?.focus();

        if (this.getAttribute('data-password') === 'required') {
          document.querySelector('#password')?.setAttribute('required', 'required');
        }
      } else {
        if (document.querySelector('.admin.edit.user')) {
          document.querySelector('#user_name')?.setAttribute('disabled', 'disabled');
        }
        document.querySelector('#login_name')?.setAttribute('required', 'required');
        showElem('.non-local');
        hideElem('.local');
        document.querySelector('#login_name')?.focus();

        document.querySelector('#password')?.removeAttribute('required');
      }
    });
  }

  function onUsePagedSearchChange() {
    const searchPageSizeElements = document.querySelectorAll('.search-page-size');
    if (document.querySelector('#use_paged_search').checked) {
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

    const provider = document.querySelector('#oauth2_provider').value;
    switch (provider) {
      case 'openidConnect':
        document.querySelector('.open_id_connect_auto_discovery_url input').setAttribute('required', 'required');
        showElem('.open_id_connect_auto_discovery_url');
        break;
      default: {
        const elProviderCustomUrlSettings = document.querySelector(`#${provider}_customURLSettings`);
        if (!elProviderCustomUrlSettings) break; // some providers do not have custom URL settings
        const couldChangeCustomURLs = elProviderCustomUrlSettings.getAttribute('data-available') === 'true';
        const mustProvideCustomURLs = elProviderCustomUrlSettings.getAttribute('data-required') === 'true';
        if (couldChangeCustomURLs) {
          showElem('.oauth2_use_custom_url'); // show the checkbox
        }
        if (mustProvideCustomURLs) {
          document.querySelector('#oauth2_use_custom_url').checked = true; // make the checkbox checked
        }
        break;
      }
    }
    onOAuth2UseCustomURLChange(applyDefaultValues);
  }

  function onOAuth2UseCustomURLChange(applyDefaultValues) {
    const provider = document.querySelector('#oauth2_provider').value;
    hideElem('.oauth2_use_custom_url_field');
    for (const input of document.querySelectorAll('.oauth2_use_custom_url_field input[required]')) {
      input.removeAttribute('required');
    }

    const elProviderCustomUrlSettings = document.querySelector(`#${provider}_customURLSettings`);
    if (elProviderCustomUrlSettings && document.querySelector('#oauth2_use_custom_url').checked) {
      for (const custom of ['token_url', 'auth_url', 'profile_url', 'email_url', 'tenant']) {
        if (applyDefaultValues) {
          document.querySelector(`#oauth2_${custom}`).value = document.querySelector(`#${provider}_${custom}`).value;
        }
        const customInput = document.querySelector(`#${provider}_${custom}`);
        if (customInput && customInput.getAttribute('data-available') === 'true') {
          for (const input of document.querySelectorAll(`.oauth2_${custom} input`)) {
            input.setAttribute('required', 'required');
          }
          showElem(`.oauth2_${custom}`);
        }
      }
    }
  }

  function onEnableLdapGroupsChange() {
    toggleElem(document.querySelector('#ldap-group-options'), $('.js-ldap-group-toggle')[0].checked);
  }

  // New authentication
  if (document.querySelector('.admin.new.authentication')) {
    document.querySelector('#auth_type')?.addEventListener('change', function () {
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
    document.querySelector('#security_protocol')?.addEventListener('change', onSecurityProtocolChange);
    document.querySelector('#use_paged_search')?.addEventListener('change', onUsePagedSearchChange);
    document.querySelector('#oauth2_provider')?.addEventListener('change', () => onOAuth2Change(true));
    document.querySelector('#oauth2_use_custom_url')?.addEventListener('change', () => onOAuth2UseCustomURLChange(true));
    $('.js-ldap-group-toggle').on('change', onEnableLdapGroupsChange);
  }
  // Edit authentication
  if (document.querySelector('.admin.edit.authentication')) {
    const authType = document.querySelector('#auth_type')?.value;
    if (authType === '2' || authType === '5') {
      document.querySelector('#security_protocol')?.addEventListener('change', onSecurityProtocolChange);
      $('.js-ldap-group-toggle').on('change', onEnableLdapGroupsChange);
      onEnableLdapGroupsChange();
      if (authType === '2') {
        document.querySelector('#use_paged_search')?.addEventListener('change', onUsePagedSearchChange);
      }
    } else if (authType === '6') {
      document.querySelector('#oauth2_provider')?.addEventListener('change', () => onOAuth2Change(true));
      document.querySelector('#oauth2_use_custom_url')?.addEventListener('change', () => onOAuth2UseCustomURLChange(false));
      onOAuth2Change(false);
    }
  }

  if (document.querySelector('.admin.authentication')) {
    $('#auth_name').on('input', function () {
      // appSubUrl is either empty or is a path that starts with `/` and doesn't have a trailing slash.
      document.querySelector('#oauth2-callback-url').textContent = `${window.location.origin}${appSubUrl}/user/oauth2/${encodeURIComponent(this.value)}/callback`;
    }).trigger('input');
  }

  // Notice
  if (document.querySelector('.admin.notice')) {
    const detailModal = document.querySelector('#detail-modal');

    // Attach view detail modals
    $('.view-detail').on('click', function () {
      const description = this.closest('tr').querySelector('.notice-description').textContent;
      detailModal.querySelector('.content pre').textContent = description;
      $(detailModal).modal('show');
      return false;
    });

    // Select actions
    const checkboxes = document.querySelectorAll('.select.table .ui.checkbox input');

    $('.select.action').on('click', function () {
      switch ($(this).data('action')) {
        case 'select-all':
          for (const checkbox of checkboxes) {
            checkbox.checked = true;
          }
          break;
        case 'deselect-all':
          for (const checkbox of checkboxes) {
            checkbox.checked = false;
          }
          break;
        case 'inverse':
          for (const checkbox of checkboxes) {
            checkbox.checked = !checkbox.checked;
          }
          break;
      }
    });
    document.querySelector('#delete-selection')?.addEventListener('click', async function (e) {
      e.preventDefault();
      this.classList.add('is-loading', 'disabled');
      const data = new FormData();
      for (const checkbox of checkboxes) {
        if (checkbox.checked) {
          data.append('ids[]', checkbox.closest('.ui.checkbox').getAttribute('data-id'));
        }
      }
      await POST(this.getAttribute('data-link'), {data});
      window.location.href = this.getAttribute('data-redirect');
    });
  }
}
