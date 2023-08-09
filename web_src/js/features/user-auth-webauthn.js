import {encodeURLEncodedBase64, decodeURLEncodedBase64} from '../utils.js';
import {showElem} from '../utils/dom.js';

const {appSubUrl, csrfToken} = window.config;

export async function initUserAuthWebAuthn() {
  const elPrompt = document.querySelector('.user.signin.webauthn-prompt');
  if (!elPrompt) {
    return;
  }

  if (!detectWebAuthnSupport()) {
    return;
  }

  const res = await fetch(`${appSubUrl}/user/webauthn/assertion`);
  if (res.status !== 200) {
    webAuthnError('unknown');
    return;
  }
  const options = await res.json();
  options.publicKey.challenge = decodeURLEncodedBase64(options.publicKey.challenge);
  for (const cred of options.publicKey.allowCredentials) {
    cred.id = decodeURLEncodedBase64(cred.id);
  }
  try {
    const credential = await navigator.credentials.get({
      publicKey: options.publicKey
    });
    await verifyAssertion(credential);
  } catch (err) {
    if (!options.publicKey.extensions?.appid) {
      webAuthnError('general', err.message);
      return;
    }
    delete options.publicKey.extensions.appid;
    try {
      const credential = await navigator.credentials.get({
        publicKey: options.publicKey
      });
      await verifyAssertion(credential);
    } catch (err) {
      webAuthnError('general', err.message);
    }
  }
}

async function verifyAssertion(assertedCredential) {
  // Move data into Arrays in case it is super long
  const authData = new Uint8Array(assertedCredential.response.authenticatorData);
  const clientDataJSON = new Uint8Array(assertedCredential.response.clientDataJSON);
  const rawId = new Uint8Array(assertedCredential.rawId);
  const sig = new Uint8Array(assertedCredential.response.signature);
  const userHandle = new Uint8Array(assertedCredential.response.userHandle);

  const res = await fetch(`${appSubUrl}/user/webauthn/assertion`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json; charset=utf-8'
    },
    body: JSON.stringify({
      id: assertedCredential.id,
      rawId: encodeURLEncodedBase64(rawId),
      type: assertedCredential.type,
      clientExtensionResults: assertedCredential.getClientExtensionResults(),
      response: {
        authenticatorData: encodeURLEncodedBase64(authData),
        clientDataJSON: encodeURLEncodedBase64(clientDataJSON),
        signature: encodeURLEncodedBase64(sig),
        userHandle: encodeURLEncodedBase64(userHandle),
      },
    }),
  });
  if (res.status === 500) {
    webAuthnError('unknown');
    return;
  } else if (res.status !== 200) {
    webAuthnError('unable-to-process');
    return;
  }
  const reply = await res.json();

  window.location.href = reply?.redirect ?? `${appSubUrl}/`;
}

async function webauthnRegistered(newCredential) {
  const attestationObject = new Uint8Array(newCredential.response.attestationObject);
  const clientDataJSON = new Uint8Array(newCredential.response.clientDataJSON);
  const rawId = new Uint8Array(newCredential.rawId);

  const res = await fetch(`${appSubUrl}/user/settings/security/webauthn/register`, {
    method: 'POST',
    headers: {
      'X-Csrf-Token': csrfToken,
      'Content-Type': 'application/json; charset=utf-8',
    },
    body: JSON.stringify({
      id: newCredential.id,
      rawId: encodeURLEncodedBase64(rawId),
      type: newCredential.type,
      response: {
        attestationObject: encodeURLEncodedBase64(attestationObject),
        clientDataJSON: encodeURLEncodedBase64(clientDataJSON),
      },
    }),
  });

  if (res.status === 409) {
    webAuthnError('duplicated');
    return;
  } else if (res.status !== 201) {
    webAuthnError('unknown');
    return;
  }

  window.location.reload();
}

function webAuthnError(errorType, message) {
  const elErrorMsg = document.getElementById(`webauthn-error-msg`);

  if (errorType === 'general') {
    elErrorMsg.textContent = message || 'unknown error';
  } else {
    const elTypedError = document.querySelector(`#webauthn-error [data-webauthn-error-msg=${errorType}]`);
    if (elTypedError) {
      elErrorMsg.textContent = `${elTypedError.textContent}${message ? ` ${message}` : ''}`;
    } else {
      elErrorMsg.textContent = `unknown error type: ${errorType}${message ? ` ${message}` : ''}`;
    }
  }

  showElem('#webauthn-error');
}

function detectWebAuthnSupport() {
  if (!window.isSecureContext) {
    webAuthnError('insecure');
    return false;
  }

  if (typeof window.PublicKeyCredential !== 'function') {
    webAuthnError('browser');
    return false;
  }

  return true;
}

export function initUserAuthWebAuthnRegister() {
  const elRegister = document.getElementById('register-webauthn');
  if (!elRegister) {
    return;
  }
  if (!detectWebAuthnSupport()) {
    elRegister.disabled = true;
    return;
  }
  elRegister.addEventListener('click', async (e) => {
    e.preventDefault();
    await webAuthnRegisterRequest();
  });
}

async function webAuthnRegisterRequest() {
  const elNickname = document.getElementById('nickname');

  const body = new FormData();
  body.append('name', elNickname.value);

  const res = await fetch(`${appSubUrl}/user/settings/security/webauthn/request_register`, {
    method: 'POST',
    headers: {
      'X-Csrf-Token': csrfToken,
    },
    body,
  });

  if (res.status === 409) {
    webAuthnError('duplicated');
    return;
  } else if (res.status !== 200) {
    webAuthnError('unknown');
    return;
  }

  const options = await res.json();
  elNickname.closest('div.field').classList.remove('error');

  options.publicKey.challenge = decodeURLEncodedBase64(options.publicKey.challenge);
  options.publicKey.user.id = decodeURLEncodedBase64(options.publicKey.user.id);
  if (options.publicKey.excludeCredentials) {
    for (const cred of options.publicKey.excludeCredentials) {
      cred.id = decodeURLEncodedBase64(cred.id);
    }
  }

  try {
    const credential = await navigator.credentials.create({
      publicKey: options.publicKey
    });
    await webauthnRegistered(credential);
  } catch (err) {
    webAuthnError('unknown', err);
  }
}
