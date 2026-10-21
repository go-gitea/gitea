import {hideElem, showElem, toggleElem} from '../utils/dom.ts';
import {GET, POST} from '../modules/fetch.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';

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
  registerGlobalInitFunc<HTMLFormElement>('initRepoMigrationForm', (form) => {
    const cloneAddrInput = form.querySelector<HTMLInputElement>('input[name="clone_addr"]')!;
    const uidInput = form.querySelector<HTMLInputElement>('input[name="uid"]')!;
    // the forge token stays visible for SSH URLs, the API calls still need it
    const userpassFields = form.querySelectorAll('.auth-userpass-field');
    const sshHelpField = form.querySelector('.ssh-help-field')!;
    const keysLink = form.querySelector<HTMLAnchorElement>('.js-ssh-keys-link')!;
    const fingerprintOnly = form.querySelector('.ssh-key-fingerprint-only')!;
    const selector = form.querySelector('.ssh-key-owner-selector')!;
    const keyOwnerInput = selector.querySelector<HTMLInputElement>('input[name="ssh_key_owner_id"]')!;
    const orgDefaultFingerprint = selector.querySelector('.menu .item[data-value="0"] .item-fingerprint')!;
    const signedUserID = selector.getAttribute('data-signed-user-id')!;
    const personalKeysURL = selector.getAttribute('data-personal-keys-link')!;
    const ownerFingerprints: Record<string, string> = JSON.parse(selector.getAttribute('data-owner-fingerprints') || '{}');
    const ownerKeysURLs: Record<string, string> = JSON.parse(selector.getAttribute('data-owner-keys-links') || '{}');

    const update = () => {
      const isSSH = isSSHURL(cloneAddrInput.value.trim());
      for (const field of userpassFields) {
        if (isSSH) {
          for (const input of field.querySelectorAll('input')) input.value = '';
        }
        toggleElem(field, !isSSH);
      }
      toggleElem(sshHelpField, isSSH);

      // only an org target offers a choice between its key and the personal one
      const isOrgTarget = isSSH && uidInput.value !== signedUserID;
      toggleElem(fingerprintOnly, isSSH && !isOrgTarget);
      toggleElem(selector, isOrgTarget);
      if (!isOrgTarget) {
        keyOwnerInput.value = '0';
        if (isSSH) keysLink.href = personalKeysURL;
        return;
      }
      orgDefaultFingerprint.textContent = ownerFingerprints[uidInput.value] ?? '';
      keysLink.href = keyOwnerInput.value === signedUserID ? personalKeysURL : (ownerKeysURLs[uidInput.value] ?? personalKeysURL);
    };

    for (const item of form.querySelectorAll('.owner.dropdown .menu .item, .ssh-key-owner-selector .menu .item')) {
      item.addEventListener('click', () => setTimeout(update, 0));
    }
    cloneAddrInput.addEventListener('input', update);
    update();
  });
}
