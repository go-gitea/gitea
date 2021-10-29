export function initInstall() {
  if ($('.install').length === 0) {
    return;
  }

  if ($('#db_host').val() === '') {
    $('#db_host').val('127.0.0.1:3306');
    $('#db_user').val('gitea');
    $('#db_name').val('gitea');
  }

  // Database type change detection.
  $('#db_type').on('change', function () {
    const sqliteDefault = 'data/gitea.db';
    const tidbDefault = 'data/gitea_tidb';

    const dbType = $(this).val();
    if (dbType === 'SQLite3') {
      $('#sql_settings').hide();
      $('#pgsql_settings').hide();
      $('#mysql_settings').hide();
      $('#sqlite_settings').show();

      if (dbType === 'SQLite3' && $('#db_path').val() === tidbDefault) {
        $('#db_path').val(sqliteDefault);
      }
      return;
    }

    const dbDefaults = {
      MySQL: '127.0.0.1:3306',
      PostgreSQL: '127.0.0.1:5432',
      MSSQL: '127.0.0.1:1433'
    };

    $('#sqlite_settings').hide();
    $('#sql_settings').show();

    $('#pgsql_settings').toggle(dbType === 'PostgreSQL');
    $('#mysql_settings').toggle(dbType === 'MySQL');
    $.each(dbDefaults, (_type, defaultHost) => {
      if ($('#db_host').val() === defaultHost) {
        $('#db_host').val(dbDefaults[dbType]);
        return false;
      }
    });
  });

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
