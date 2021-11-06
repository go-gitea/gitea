const {appSubUrl, csrfToken} = window.config;

export function initUserAuthU2fAuth() {
  if ($('#wait-for-key').length === 0) {
    return;
  }
  u2fApi.ensureSupport().then(() => {
    $.getJSON(`${appSubUrl}/user/u2f/challenge`).done((req) => {
      u2fApi.sign(req.appId, req.challenge, req.registeredKeys, 30)
        .then(u2fSigned)
        .catch((err) => {
          if (err === undefined) {
            u2fError(1);
            return;
          }
          u2fError(err.metaData.code);
        });
    });
  }).catch(() => {
    // Fallback in case browser do not support U2F
    window.location.href = `${appSubUrl}/user/two_factor`;
  });
}

function u2fSigned(resp) {
  $.ajax({
    url: `${appSubUrl}/user/u2f/sign`,
    type: 'POST',
    headers: {'X-Csrf-Token': csrfToken},
    data: JSON.stringify(resp),
    contentType: 'application/json; charset=utf-8',
  }).done((res) => {
    window.location.replace(res);
  }).fail(() => {
    u2fError(1);
  });
}

function u2fRegistered(resp) {
  if (checkError(resp)) {
    return;
  }
  $.ajax({
    url: `${appSubUrl}/user/settings/security/u2f/register`,
    type: 'POST',
    headers: {'X-Csrf-Token': csrfToken},
    data: JSON.stringify(resp),
    contentType: 'application/json; charset=utf-8',
    success() {
      window.location.reload();
    },
    fail() {
      u2fError(1);
    }
  });
}

function checkError(resp) {
  if (!('errorCode' in resp)) {
    return false;
  }
  if (resp.errorCode === 0) {
    return false;
  }
  u2fError(resp.errorCode);
  return true;
}

function u2fError(errorType) {
  const u2fErrors = {
    browser: $('#unsupported-browser'),
    1: $('#u2f-error-1'),
    2: $('#u2f-error-2'),
    3: $('#u2f-error-3'),
    4: $('#u2f-error-4'),
    5: $('.u2f_error_5')
  };
  u2fErrors[errorType].removeClass('hide');

  Object.keys(u2fErrors).forEach((type) => {
    if (type !== `${errorType}`) {
      u2fErrors[type].addClass('hide');
    }
  });
  $('#u2f-error').modal('show');
}

export function initUserAuthU2fRegister() {
  $('#register-device').modal({allowMultiple: false});
  $('#u2f-error').modal({allowMultiple: false});
  $('#register-security-key').on('click', (e) => {
    e.preventDefault();
    u2fApi.ensureSupport()
      .then(u2fRegisterRequest)
      .catch(() => {
        u2fError('browser');
      });
  });
}

function u2fRegisterRequest() {
  $.post(`${appSubUrl}/user/settings/security/u2f/request_register`, {
    _csrf: csrfToken,
    name: $('#nickname').val()
  }).done((req) => {
    $('#nickname').closest('div.field').removeClass('error');
    $('#register-device').modal('show');
    if (req.registeredKeys === null) {
      req.registeredKeys = [];
    }
    u2fApi.register(req.appId, req.registerRequests, req.registeredKeys, 30)
      .then(u2fRegistered)
      .catch((reason) => {
        if (reason === undefined) {
          u2fError(1);
          return;
        }
        u2fError(reason.metaData.code);
      });
  }).fail((xhr) => {
    if (xhr.status === 409) {
      $('#nickname').closest('div.field').addClass('error');
    }
  });
}
