import $ from 'jquery';
import {hideElem, showElem} from '../utils/dom.js';
import {GET} from '../modules/fetch.js';

export function initInstall() {
  const $page = $('.page-content.install');
  if ($page.length === 0) {
    return;
  }
  if ($page.is('.post-install')) {
    initPostInstall();
  } else {
    initPreInstall();
  }
}

function initPreInstall() {
  const defaultDbUser = 'gitea';
  const defaultDbName = 'gitea';

  const defaultDbHosts = {
    mysql: '127.0.0.1:3306',
    postgres: '127.0.0.1:5432',
    mssql: '127.0.0.1:1433'
  };

  const $dbHost = $('#db_host');
  const $dbUser = $('#db_user');
  const $dbName = $('#db_name');

  // Database type change detection.
  $('#db_type').on('change', function () {
    const dbType = $(this).val();
    hideElem($('div[data-db-setting-for]'));
    showElem($(`div[data-db-setting-for=${dbType}]`));

    if (dbType !== 'sqlite3') {
      // for most remote database servers
      showElem($(`div[data-db-setting-for=common-host]`));
      const lastDbHost = $dbHost.val();
      const isDbHostDefault = !lastDbHost || Object.values(defaultDbHosts).includes(lastDbHost);
      if (isDbHostDefault) {
        $dbHost.val(defaultDbHosts[dbType] ?? '');
      }
      if (!$dbUser.val() && !$dbName.val()) {
        $dbUser.val(defaultDbUser);
        $dbName.val(defaultDbName);
      }
    } // else: for SQLite3, the default path is always prepared by backend code (setting)
  }).trigger('change');

  const $appUrl = $('#app_url');
  const configAppUrl = $appUrl.val();
  if (configAppUrl.includes('://localhost')) {
    $appUrl.val(window.location.href);
  }

  const $domain = $('#domain');
  const configDomain = $domain.val().trim();
  if (configDomain === 'localhost') {
    $domain.val(window.location.hostname);
  }

  // TODO: better handling of exclusive relations.
  $('#offline-mode input').on('change', function () {
    if ($(this).is(':checked')) {
      $('#disable-gravatar').checkbox('check');
      $('#federated-avatar-lookup').checkbox('uncheck');
    }
  });
  $('#disable-gravatar input').on('change', function () {
    if ($(this).is(':checked')) {
      $('#federated-avatar-lookup').checkbox('uncheck');
    } else {
      $('#offline-mode').checkbox('uncheck');
    }
  });
  $('#federated-avatar-lookup input').on('change', function () {
    if ($(this).is(':checked')) {
      $('#disable-gravatar').checkbox('uncheck');
      $('#offline-mode').checkbox('uncheck');
    }
  });
  $('#enable-openid-signin input').on('change', function () {
    if ($(this).is(':checked')) {
      if (!$('#disable-registration input').is(':checked')) {
        $('#enable-openid-signup').checkbox('check');
      }
    } else {
      $('#enable-openid-signup').checkbox('uncheck');
    }
  });
  $('#disable-registration input').on('change', function () {
    if ($(this).is(':checked')) {
      $('#enable-captcha').checkbox('uncheck');
      $('#enable-openid-signup').checkbox('uncheck');
    } else {
      $('#enable-openid-signup').checkbox('check');
    }
  });
  $('#enable-captcha input').on('change', function () {
    if ($(this).is(':checked')) {
      $('#disable-registration').checkbox('uncheck');
    }
  });
}

function initPostInstall() {
  const el = document.getElementById('goto-user-login');
  if (!el) return;

  const targetUrl = el.getAttribute('href');
  let tid = setInterval(async () => {
    try {
      const resp = await GET(targetUrl);
      if (tid && resp.status === 200) {
        clearInterval(tid);
        tid = null;
        window.location.href = targetUrl;
      }
    } catch {}
  }, 1000);
}
