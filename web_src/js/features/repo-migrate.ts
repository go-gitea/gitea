import {hideElem, showElem, type DOMEvent} from '../utils/dom.ts';
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
      document.querySelector('#repo_migrating_progress_message').textContent = data.message;
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
      document.querySelector('#repo_migrating_failed_error').textContent = data.message;
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

async function doMigrationRetry(e: DOMEvent<MouseEvent>) {
  await POST(e.target.getAttribute('data-migrating-task-retry-url'));
  window.location.reload();
}

export function initRepoMigrationForm() {
  const cloneAddrInput = document.querySelector<HTMLInputElement>('#clone_addr');
  const authUsernameInput = document.querySelector<HTMLInputElement>('#auth_username');
  const authPasswordInput = document.querySelector<HTMLInputElement>('#auth_password');
  const sshHelpText = document.querySelector('.help.ssh-help');

  if (!cloneAddrInput || !authUsernameInput || !authPasswordInput || !sshHelpText) return;

  function isSSHURL(url: string): boolean {
    return url.startsWith('ssh://') ||
           url.startsWith('git@') ||
           (url.includes('@') && url.includes(':') && !url.includes('://'));
  }

  function updateAuthFields() {
    const url = cloneAddrInput.value.trim();
    const isSSH = isSSHURL(url);

    if (isSSH) {
      // Disable auth fields for SSH URLs
      authUsernameInput.disabled = true;
      authPasswordInput.disabled = true;
      authUsernameInput.value = '';
      authPasswordInput.value = '';
      authUsernameInput.parentElement?.classList.add('disabled');
      authPasswordInput.parentElement?.classList.add('disabled');
      showElem(sshHelpText);
    } else {
      authUsernameInput.disabled = false;
      authPasswordInput.disabled = false;
      authUsernameInput.parentElement?.classList.remove('disabled');
      authPasswordInput.parentElement?.classList.remove('disabled');
      hideElem(sshHelpText);
    }
  }

  updateAuthFields();
  cloneAddrInput.addEventListener('input', updateAuthFields);
  cloneAddrInput.addEventListener('blur', updateAuthFields);
}
