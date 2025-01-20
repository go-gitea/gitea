import {encodeURLEncodedBase64, decodeURLEncodedBase64} from '../utils.ts';
import {showElem} from '../utils/dom.ts';
import {GET, POST} from '../modules/fetch.ts';

const {appSubUrl} = window.config;

export async function initUserAuthWebAuthn() {
  const elPrompt = document.querySelector('.user.signin.webauthn-prompt');
  const elSignInPasskeyBtn = document.querySelector('.signin-passkey');
  if (!elPrompt && !elSignInPasskeyBtn) {
    return;
  }

  if (!detectWebAuthnSupport()) {
    return;
  }

  if (elSignInPasskeyBtn) {
    elSignInPasskeyBtn.addEventListener('click', loginPasskey);
  }

  if (elPrompt) {
    login2FA();
  }
}

async function loginPasskey() {
  const res = await GET(`${appSubUrl}/user/webauthn/passkey/assertion`);
  if (!res.ok) {
    webAuthnError('unknown');
    return;
  }

  const options = await res.json();
  options.publicKey.challenge = decodeURLEncodedBase64(options.publicKey.challenge);
  for (const cred of options.publicKey.allowCredentials ?? []) {
    cred.id = decodeURLEncodedBase64(cred.id);
  }

  try {
    const credential = await navigator.credentials.get({
      publicKey: options.publicKey,
    }) as PublicKeyCredential;
    const credResp = credential.response as AuthenticatorAssertionResponse;

    // Move data into Arrays in case it is super long
    const authData = new Uint8Array(credResp.authenticatorData);
    const clientDataJSON = new Uint8Array(credResp.clientDataJSON);
    const rawId = new Uint8Array(credential.rawId);
    const sig = new Uint8Array(credResp.signature);
    const userHandle = new Uint8Array(credResp.userHandle);

    const res = await POST(`${appSubUrl}/user/webauthn/passkey/login`, {
      data: {
        id: credential.id,
        rawId: encodeURLEncodedBase64(rawId),
        type: credential.type,
        clientExtensionResults: credential.getClientExtensionResults(),
        response: {
          authenticatorData: encodeURLEncodedBase64(authData),
          clientDataJSON: encodeURLEncodedBase64(clientDataJSON),
          signature: encodeURLEncodedBase64(sig),
          userHandle: encodeURLEncodedBase64(userHandle),
        },
      },
    });
    if (res.status === 500) {
      webAuthnError('unknown');
      return;
    } else if (!res.ok) {
      webAuthnError('unable-to-process');
      return;
    }
    const reply = await res.json();

    window.location.href = reply?.redirect ?? `${appSubUrl}/`;
  } catch (err) {
    webAuthnError('general', err.message);
  }
}

async function login2FA() {
  const res = await GET(`${appSubUrl}/user/webauthn/assertion`);
  if (!res.ok) {
    webAuthnError('unknown');
    return;
  }

  const options = await res.json();
  options.publicKey.challenge = decodeURLEncodedBase64(options.publicKey.challenge);
  for (const cred of options.publicKey.allowCredentials ?? []) {
    cred.id = decodeURLEncodedBase64(cred.id);
  }

  try {
    const credential = await navigator.credentials.get({
      publicKey: options.publicKey,
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
        publicKey: options.publicKey,
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

  const res = await POST(`${appSubUrl}/user/webauthn/assertion`, {
    data: {
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
    },
  });
  if (res.status === 500) {
    webAuthnError('unknown');
    return;
  } else if (!res.ok) {
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

  const res = await POST(`${appSubUrl}/user/settings/security/webauthn/register`, {
    data: {
      id: newCredential.id,
      rawId: encodeURLEncodedBase64(rawId),
      type: newCredential.type,
      response: {
        attestationObject: encodeURLEncodedBase64(attestationObject),
        clientDataJSON: encodeURLEncodedBase64(clientDataJSON),
      },
    },
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

function webAuthnError(errorType: string, message:string = '') {
  const elErrorMsg = document.querySelector(`#webauthn-error-msg`);

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
  const elRegister = document.querySelector<HTMLInputElement>('#register-webauthn');
  if (!elRegister) return;

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
  const elNickname = document.querySelector<HTMLInputElement>('#nickname');

  const formData = new FormData();
  formData.append('name', elNickname.value);

  const res = await POST(`${appSubUrl}/user/settings/security/webauthn/request_register`, {
    data: formData,
  });

  if (res.status === 409) {
    webAuthnError('duplicated');
    return;
  } else if (!res.ok) {
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
      publicKey: options.publicKey,
    });
    await webauthnRegistered(credential);
  } catch (err) {
    webAuthnError('unknown', err);
  }
}
