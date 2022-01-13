import {encode} from 'uint8-to-base64';

const {appSubUrl, csrfToken} = window.config;


export function initUserAuthWebAuthn() {
  if ($('.user.signin.webauthn-prompt').length === 0) {
    return;
  }

  if (!detectWebAuthnSupport()) {
    return;
  }

  $.getJSON(`${appSubUrl}/user/webauthn/assertion`, {})
    .done((makeAssertionOptions) => {
      makeAssertionOptions.publicKey.challenge = bufferDecode(makeAssertionOptions.publicKey.challenge);
      for (let i = 0; i < makeAssertionOptions.publicKey.allowCredentials.length; i++) {
        makeAssertionOptions.publicKey.allowCredentials[i].id = bufferDecode(makeAssertionOptions.publicKey.allowCredentials[i].id);
      }
      navigator.credentials.get({
        publicKey: makeAssertionOptions.publicKey
      })
        .then((credential) => {
          verifyAssertion(credential);
        }).catch((err) => {
          webAuthnError(0, err.message);
        });
    }).fail(() => {
      webAuthnError('unknown');
    });
}

function verifyAssertion(assertedCredential) {
  // Move data into Arrays incase it is super long
  const authData = new Uint8Array(assertedCredential.response.authenticatorData);
  const clientDataJSON = new Uint8Array(assertedCredential.response.clientDataJSON);
  const rawId = new Uint8Array(assertedCredential.rawId);
  const sig = new Uint8Array(assertedCredential.response.signature);
  const userHandle = new Uint8Array(assertedCredential.response.userHandle);
  $.ajax({
    url: `${appSubUrl}/user/webauthn/assertion`,
    type: 'POST',
    data: JSON.stringify({
      id: assertedCredential.id,
      rawId: bufferEncode(rawId),
      type: assertedCredential.type,
      response: {
        authenticatorData: bufferEncode(authData),
        clientDataJSON: bufferEncode(clientDataJSON),
        signature: bufferEncode(sig),
        userHandle: bufferEncode(userHandle),
      },
    }),
    contentType: 'application/json; charset=utf-8',
    dataType: 'json',
    success: (resp) => {
      if (resp && resp['redirect']) {
        window.location.href = resp['redirect'];
      } else {
        window.location.href = '/';
      }
    },
    error: (xhr) => {
      if (xhr.status === 500) {
        webAuthnError('unknown');
        return;
      }
      webAuthnError('unable-to-process');
    }
  });
}

// Encode an ArrayBuffer into a base64 string.
function bufferEncode(value) {
  return encode(value)
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '');
}

function webauthnRegistered(newCredential) {
  const attestationObject = new Uint8Array(newCredential.response.attestationObject);
  const clientDataJSON = new Uint8Array(newCredential.response.clientDataJSON);
  const rawId = new Uint8Array(newCredential.rawId);

  return $.ajax({
    url: `${appSubUrl}/user/settings/security/webauthn/register`,
    type: 'POST',
    headers: {'X-Csrf-Token': csrfToken},
    data: JSON.stringify({
      id: newCredential.id,
      rawId: bufferEncode(rawId),
      type: newCredential.type,
      response: {
        attestationObject: bufferEncode(attestationObject),
        clientDataJSON: bufferEncode(clientDataJSON),
      },
    }),
    dataType: 'json',
    contentType: 'application/json; charset=utf-8',
  }).then(() => {
    window.location.reload();
  }).fail((xhr) => {
    if (xhr.status === 409) {
      webAuthnError('duplicated');
      return;
    }
    webAuthnError('unknown');
  });
}

function webAuthnError(errorType, message) {
  $('#webauthn-error [data-webauthn-error-msg]').hide();
  if (errorType === 0 && message && message.length > 1) {
    $(`#webauthn-error [data-webauthn-error-msg=0]`).text(message);
    $(`#webauthn-error [data-webauthn-error-msg=0]`).show();
  } else {
    $(`#webauthn-error [data-webauthn-error-msg=${errorType}]`).show();
  }
  $('#webauthn-error').modal('show');
}

function detectWebAuthnSupport() {
  if (typeof window.PublicKeyCredential !== 'function') {
    $('#register-button').prop('disabled', true);
    $('#login-button').prop('disabled', true);
    webAuthnError('browser');
    return false;
  }

  if (window.location.protocol === 'http:' && (window.location.hostname !== 'localhost' && window.location.hostname !== '127.0.0.1')) {
    $('#register-button').prop('disabled', true);
    $('#login-button').prop('disabled', true);
    webAuthnError('insecure');
    return false;
  }
  return true;
}

export function initUserAuthWebAuthnRegister() {
  if ($('#register-webauthn').length === 0) {
    return;
  }

  if (!detectWebAuthnSupport()) {
    return;
  }

  $('#register-device').modal({allowMultiple: false});
  $('#webauthn-error').modal({allowMultiple: false});
  $('#register-webauthn').on('click', (e) => {
    e.preventDefault();
    webAuthnRegisterRequest();
  });
}

function bufferDecode(value) {
  return Uint8Array.from(atob(value), (c) => c.codePointAt(0));
}

function webAuthnRegisterRequest() {
  if ($('#nickname').val() === '') {
    webAuthnError('empty');
    return;
  }
  $.post(`${appSubUrl}/user/settings/security/webauthn/request_register`, {
    _csrf: csrfToken,
    name: $('#nickname').val(),
  }).done((makeCredentialOptions) => {
    $('#nickname').closest('div.field').removeClass('error');
    $('#register-device').modal('show');

    makeCredentialOptions.publicKey.challenge = bufferDecode(makeCredentialOptions.publicKey.challenge);
    makeCredentialOptions.publicKey.user.id = bufferDecode(makeCredentialOptions.publicKey.user.id);
    if (makeCredentialOptions.publicKey.excludeCredentials) {
      for (let i = 0; i < makeCredentialOptions.publicKey.excludeCredentials.length; i++) {
        makeCredentialOptions.publicKey.excludeCredentials[i].id = bufferDecode(makeCredentialOptions.publicKey.excludeCredentials[i].id);
      }
    }

    navigator.credentials.create({
      publicKey: makeCredentialOptions.publicKey
    }).then(webauthnRegistered)
      .catch((err) => {
        if (err === undefined) {
          webAuthnError('unknown');
          return;
        }
        webAuthnError(0, err);
      });
  }).fail((xhr) => {
    if (xhr.status === 409) {
      webAuthnError('duplicated');
      return;
    }
    webAuthnError('unknown');
  });
}
