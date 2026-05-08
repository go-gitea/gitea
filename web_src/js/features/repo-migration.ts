import {hideElem, showElem, toggleElem} from '../utils/dom.ts';
import {GET, POST} from '../modules/fetch.ts';
import {sanitizeRepoName} from './repo-common.ts';

const service = document.querySelector<HTMLInputElement>('#service_type');
const user = document.querySelector<HTMLInputElement>('#auth_username');
const pass = document.querySelector<HTMLInputElement>('#auth_password');
const token = document.querySelector<HTMLInputElement>('#auth_token');
const mirror = document.querySelector<HTMLInputElement>('#mirror');
const lfs = document.querySelector<HTMLInputElement>('#lfs');
const lfsSettings = document.querySelector<HTMLElement>('#lfs_settings')!;
const lfsEndpoint = document.querySelector<HTMLElement>('#lfs_endpoint')!;
const items = document.querySelectorAll<HTMLInputElement>('#migrate_items input[type=checkbox]');

export function initRepoMigration() {
  checkAuth();
  setLFSSettingsVisibility();

  user?.addEventListener('input', () => {checkItems(false)});
  pass?.addEventListener('input', () => {checkItems(false)});
  token?.addEventListener('input', () => {checkItems(true)});
  mirror?.addEventListener('change', () => {checkItems(true)});
  document.querySelector('#lfs_settings_show')?.addEventListener('click', (e) => {
    e.preventDefault();
    e.stopPropagation();
    showElem(lfsEndpoint);
  });
  lfs?.addEventListener('change', setLFSSettingsVisibility);

  const elCloneAddr = document.querySelector<HTMLInputElement>('#clone_addr');
  const elRepoName = document.querySelector<HTMLInputElement>('#repo_name');
  if (elCloneAddr && elRepoName) {
    let repoNameChanged = false;
    elRepoName.addEventListener('input', () => {repoNameChanged = true});
    elCloneAddr.addEventListener('input', () => {
      if (repoNameChanged) return;
      let repoNameFromUrl = elCloneAddr.value.split(/[?#]/)[0];
      const parts = /^(.*\/)?((.+?)\/?)$/.exec(repoNameFromUrl);
      if (!parts || parts.length < 4) {
        elRepoName.value = '';
        return;
      }
      repoNameFromUrl = parts[3].split(/[?#]/)[0];
      elRepoName.value = sanitizeRepoName(repoNameFromUrl);
    });
  }
}

function checkAuth() {
  if (!service) return;
  const serviceType = Number(service.value);

  checkItems(serviceType !== 1);
}

function checkItems(tokenAuth: boolean) {
  let enableItems: boolean;
  if (tokenAuth) {
    enableItems = token?.value !== '';
  } else {
    enableItems = user?.value !== '' || pass?.value !== '';
  }
  if (enableItems && Number(service?.value) > 1) {
    if (mirror?.checked) {
      for (const item of items) {
        item.disabled = item.name !== 'wiki';
      }
      return;
    }
    for (const item of items) item.disabled = false;
  } else {
    for (const item of items) item.disabled = true;
  }
}

function setLFSSettingsVisibility() {
  if (!lfs) return;
  const visible = lfs.checked;
  toggleElem(lfsSettings, visible);
  hideElem(lfsEndpoint);
}

export function initRepoMigrate() {
  const authMethodDropdown = document.querySelector<HTMLInputElement>('input[name="auth_method"]');
  if (!authMethodDropdown) return;

  const authTokenField = document.querySelector<HTMLElement>('#auth_token_field');
  const githubAppField = document.querySelector<HTMLElement>('#github_app_field');

  function updateAuthFields() {
    const authMethod = authMethodDropdown?.value || 'token';

    if (authMethod === 'github_app') {
      authTokenField?.style.setProperty('display', 'none');
      githubAppField?.style.removeProperty('display');
    } else {
      authTokenField?.style.removeProperty('display');
      githubAppField?.style.setProperty('display', 'none');
    }
  }

  // Initialize on page load
  updateAuthFields();

  // Listen for changes
  authMethodDropdown.addEventListener('change', updateAuthFields);
}

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
