import {hideElem, showElem} from '../utils/dom.ts';
import {GET, POST} from '../modules/fetch.ts';

export function initRepoMigrationStatusChecker() {
  const repoMigrating = document.querySelector('#repo_migrating');
  if (!repoMigrating) return;

  document.querySelector<HTMLButtonElement>('#repo_migrating_retry')?.addEventListener('click', doMigrationRetry);

  const repoLink = repoMigrating.getAttribute('data-migrating-repo-link');

  // returns true if the refresh still needs to be called after a while
  const refresh = async () => {
    const res = await GET(`${repoLink}/-/migrate/status`);
    if (res.status !== 200) return true; // continue to refresh if network error occurs

    const data = await res.json();

    // for all status
    if (data.message) {
      document.querySelector('#repo_migrating_progress_message')!.textContent = data.message;
    }

    // TaskStatusFinished
    if (data.status === 4) {
      window.location.reload();
      return false;
    }

    // TaskStatusFailed
    if (data.status === 3) {
      hideElem('#repo_migrating_progress');
      hideElem('#repo_migrating');
      showElem('#repo_migrating_retry');
      showElem('#repo_migrating_failed');
      showElem('#repo_migrating_failed_image');
      document.querySelector('#repo_migrating_failed_error')!.textContent = data.message;
      return false;
    }

    return true; // continue to refresh
  };

  const syncTaskStatus = async () => {
    let doNextRefresh = true;
    try {
      doNextRefresh = await refresh();
    } finally {
      if (doNextRefresh) {
        setTimeout(syncTaskStatus, 2000);
      }
    }
  };

  syncTaskStatus(); // no await
}

async function doMigrationRetry(e: Event) {
  await POST((e.target as HTMLElement).getAttribute('data-migrating-task-retry-url')!);
  window.location.reload();
}

function isSSHURL(url: string): boolean {
  return url.startsWith('ssh://') ||
         url.startsWith('git@') ||
         (url.includes('@') && url.includes(':') && !url.includes('://'));
}

export function initRepoMigrationForm() {
  const cloneAddrInput = document.querySelector<HTMLInputElement>('#clone_addr');
  if (!cloneAddrInput) return;

  // SSH URLs use key-based auth, so username/password fields become useless
  // and are hidden. Forge token fields stay visible — the token is still
  // needed for API calls (issues/PRs/etc.) regardless of the git transport.
  const userpassFields = document.querySelectorAll<HTMLElement>('.auth-userpass-field');
  const sshHelpField = document.querySelector<HTMLElement>('.ssh-help-field');

  function updateAuthFields() {
    const isSSH = isSSHURL(cloneAddrInput!.value.trim());

    for (const field of userpassFields) {
      if (isSSH) {
        for (const input of field.querySelectorAll<HTMLInputElement>('input')) input.value = '';
        hideElem(field);
      } else {
        showElem(field);
      }
    }
    if (sshHelpField) {
      if (isSSH) showElem(sshHelpField); else hideElem(sshHelpField);
    }
  }

  updateAuthFields();
  cloneAddrInput.addEventListener('input', updateAuthFields);
  cloneAddrInput.addEventListener('blur', updateAuthFields);

  initSSHKeyOwnerSelector(cloneAddrInput);
}

// initSSHKeyOwnerSelector wires the managed SSH key UI on the migrate form.
// For SSH URLs it shows either a single fingerprint line (personal target —
// no choice) or a dropdown with "this org's key" vs "your personal key", and
// updates the org default item with the current org's fingerprint so the user
// always sees which key will be used. For non-SSH URLs everything stays hidden.
function initSSHKeyOwnerSelector(cloneAddrInput: HTMLInputElement) {
  const selector = document.querySelector<HTMLElement>('.ssh-key-owner-selector');
  const fingerprintOnly = document.querySelector<HTMLElement>('.ssh-key-fingerprint-only');
  const hiddenId = document.querySelector<HTMLInputElement>('#ssh_key_owner_id');
  const uidInput = document.querySelector<HTMLInputElement>('#uid');
  if (!selector || !hiddenId || !uidInput) return;

  const signedUserID = selector.getAttribute('data-signed-user-id') ?? '';
  const ownerFingerprints: Record<string, string> = JSON.parse(selector.getAttribute('data-owner-fingerprints') || '{}');
  const ownerKeysURLs: Record<string, string> = JSON.parse(selector.getAttribute('data-owner-keys-links') || '{}');
  const personalKeysURL = selector.getAttribute('data-personal-keys-link') ?? '';
  const orgDefaultFingerprintEl = selector.querySelector<HTMLElement>('.menu .item[data-value="0"] .item-fingerprint');
  const keysLink = document.querySelector<HTMLAnchorElement>('.js-ssh-keys-link');

  function update() {
    const isSSH = isSSHURL(cloneAddrInput.value.trim());
    const targetUid = uidInput!.value;

    if (!isSSH) {
      hideElem(selector!);
      if (fingerprintOnly) hideElem(fingerprintOnly);
      hiddenId!.value = '0';
      return;
    }

    // Personal target — no choice to make, just surface the fingerprint.
    if (targetUid === signedUserID) {
      hideElem(selector!);
      if (fingerprintOnly) showElem(fingerprintOnly);
      hiddenId!.value = '0';
      if (keysLink) keysLink.href = personalKeysURL;
      return;
    }

    // Org target — show dropdown; populate the org-default item's fingerprint.
    if (fingerprintOnly) hideElem(fingerprintOnly);
    if (orgDefaultFingerprintEl) orgDefaultFingerprintEl.textContent = ownerFingerprints[targetUid] ?? '';
    // Point the link at the page holding the key that will actually be used:
    // the org's keys page for the org default, or the personal keys page.
    if (keysLink) {
      keysLink.href = hiddenId!.value === signedUserID ? personalKeysURL : (ownerKeysURLs[targetUid] ?? personalKeysURL);
    }
    showElem(selector!);
  }

  for (const item of document.querySelectorAll<HTMLElement>('.owner.dropdown .menu .item, .ssh-key-owner-selector .menu .item')) {
    item.addEventListener('click', () => setTimeout(update, 0));
  }

  cloneAddrInput.addEventListener('input', update);
  cloneAddrInput.addEventListener('blur', update);

  update();
}
