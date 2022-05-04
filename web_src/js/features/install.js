import $ from 'jquery';

export function initInstall() {
  if ($('.page-content.install').length === 0) {
    return;
  }

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
    $('div[data-db-setting-for]').hide();
    $(`div[data-db-setting-for=${dbType}]`).show();

    if (dbType !== 'sqlite3') {
      // for most remote database servers
      $(`div[data-db-setting-for=common-host]`).show();
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
