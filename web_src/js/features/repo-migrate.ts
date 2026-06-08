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
  const authUsernameInput = document.querySelector<HTMLInputElement>('#auth_username');
  const authPasswordInput = document.querySelector<HTMLInputElement>('#auth_password');
  const sshHelpText = document.querySelector('.help.ssh-help');

  if (!cloneAddrInput || !authUsernameInput || !authPasswordInput || !sshHelpText) return;

  function updateAuthFields() {
    const url = cloneAddrInput!.value.trim();
    const isSSH = isSSHURL(url);

    const usernameField = authUsernameInput!.parentElement!;
    const passwordField = authPasswordInput!.parentElement!;

    if (isSSH) {
      // Hide auth fields entirely for SSH URLs (key-based auth only)
      authUsernameInput!.value = '';
      authPasswordInput!.value = '';
      hideElem(usernameField);
      hideElem(passwordField);
      showElem(sshHelpText!);
    } else {
      showElem(usernameField);
      showElem(passwordField);
      hideElem(sshHelpText!);
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
  const select = document.querySelector<HTMLSelectElement>('#ssh_key_owner_select');
  const hiddenId = document.querySelector<HTMLInputElement>('#ssh_key_owner_id');
  const uidInput = document.querySelector<HTMLInputElement>('#uid');
  if (!container || !select || !hiddenId || !uidInput) return;

  const signedUserID = container.getAttribute('data-signed-user-id') ?? '';
  const signedUserName = container.getAttribute('data-signed-user-name') ?? '';

  // Build {ownerID -> name} from the owner dropdown menu items
  const ownerNameById = new Map<string, string>();
  for (const item of document.querySelectorAll<HTMLElement>('.owner.dropdown .menu .item')) {
    const id = item.getAttribute('data-value');
    const name = item.getAttribute('title') ?? item.textContent?.trim() ?? '';
    if (id) ownerNameById.set(id, name);
  }

  function update() {
    const isSSH = isSSHURL(cloneAddrInput.value.trim());
    const targetUid = uidInput!.value;

    // No choice: non-SSH URL, or migrating into the user's own account
    if (!isSSH || targetUid === signedUserID) {
      hideElem(container!);
      hiddenId!.value = '0';
      return;
    }

    // Target is an organisation — offer both keys
    const orgName = ownerNameById.get(targetUid) ?? `#${targetUid}`;
    select!.innerHTML = '';
    select!.add(new Option(`Use ${orgName}'s managed SSH key (default)`, '0'));
    select!.add(new Option(`Use your personal managed SSH key (${signedUserName})`, signedUserID));
    select!.value = hiddenId!.value || '0';
    hiddenId!.value = select!.value;
    showElem(container!);
  }

  select.addEventListener('change', () => {
    hiddenId.value = select.value;
  });

  // Semantic UI updates the #uid hidden input via menu item clicks
  for (const item of document.querySelectorAll<HTMLElement>('.owner.dropdown .menu .item')) {
    item.addEventListener('click', () => setTimeout(update, 0));
  }

  cloneAddrInput.addEventListener('input', update);
  cloneAddrInput.addEventListener('blur', update);

  update();
}
