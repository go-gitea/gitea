const {AppSubUrl, csrf} = window.config;

export async function initU2FAuth() {
  if ($('#wait-for-key').length === 0) {
    return;
  }
  try {
    await u2fApi.ensureSupport();
  } catch (e) {
    // Fallback in case browser do not support U2F
    window.location.href = `${AppSubUrl}/user/two_factor`;
  }
  try {
    const response = await fetch(`${AppSubUrl}/user/u2f/challenge`);
    if (!response.ok) throw new Error('cannot retrieve challenge');
    const {appId, challenge, registeredKeys} = await response.json();
    const signature = await u2fApi.sign(appId, challenge, registeredKeys, 30);
    u2fSigned(signature);
  } catch (e) {
    if (e === undefined || e.metaData === undefined) {
      u2fError(1);
      return;
    }
    u2fError(e.metaData.code);
  }
}
function u2fSigned(resp) {
  $.ajax({
    url: `${AppSubUrl}/user/u2f/sign`,
    type: 'POST',
    headers: {'X-Csrf-Token': csrf},
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
    url: `${AppSubUrl}/user/settings/security/u2f/register`,
    type: 'POST',
    headers: {'X-Csrf-Token': csrf},
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
    5: $('.u2f-error-5')
  };
  u2fErrors[errorType].removeClass('hide');

  Object.keys(u2fErrors).forEach((type) => {
    if (type !== errorType) {
      u2fErrors[type].addClass('hide');
    }
  });
  $('#u2f-error').modal('show');
}

export function initU2FRegister() {
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

async function u2fRegisterRequest() {
  const body = new FormData();
  body.append('_csrf', csrf);
  body.append('name', $('#nickname').val());
  const response = await fetch(`${AppSubUrl}/user/settings/security/u2f/request_register`, {
    method: 'POST',
    body,
  });
  if (!response.ok) {
    if (response.status === 409) {
      $('#nickname').closest('div.field').addClass('error');
      return;
    }
    throw new Error('request register failed');
  }
  let {appId, registerRequests, registeredKeys} = await response.json();
  $('#nickname').closest('div.field').removeClass('error');
  $('#register-device').modal('show');
  if (registeredKeys === null) {
    registeredKeys = [];
  }
  u2fApi.register(appId, registerRequests, registeredKeys, 30)
    .then(u2fRegistered)
    .catch((reason) => {
      if (reason === undefined) {
        u2fError(1);
        return;
      }
      u2fError(reason.metaData.code);
    });
}
