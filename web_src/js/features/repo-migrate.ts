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

// initSSHKeyOwnerSelector wires the "managed SSH key owner" selector. It is
// hidden by default and only shown when an SSH URL is entered AND the chosen
// target owner is an organisation (i.e. not the signed-in user) — in that case
// the user can pick between the org's managed key (default) and their personal
// managed key. The hidden #ssh_key_owner_id field is submitted with the form.
function initSSHKeyOwnerSelector(cloneAddrInput: HTMLInputElement) {
  const container = document.querySelector<HTMLElement>('.ssh-key-owner-selector');
  const hiddenId = document.querySelector<HTMLInputElement>('#ssh_key_owner_id');
  const uidInput = document.querySelector<HTMLInputElement>('#uid');
  if (!container || !hiddenId || !uidInput) return;

  const signedUserID = container.getAttribute('data-signed-user-id') ?? '';

  function update() {
    const isSSH = isSSHURL(cloneAddrInput.value.trim());
    const targetUid = uidInput!.value;

    // No choice: non-SSH URL, or migrating into the user's own account
    if (!isSSH || targetUid === signedUserID) {
      hideElem(container!);
      hiddenId!.value = '0';
      return;
    }

    // Target is an organisation — show selector (Fomantic dropdown wires the hidden input itself)
    showElem(container!);
  }

  // Semantic UI updates the #uid hidden input via menu item clicks
  for (const item of document.querySelectorAll<HTMLElement>('.owner.dropdown .menu .item')) {
    item.addEventListener('click', () => setTimeout(update, 0));
  }

  cloneAddrInput.addEventListener('input', update);
  cloneAddrInput.addEventListener('blur', update);

  update();
}
