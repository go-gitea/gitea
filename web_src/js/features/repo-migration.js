import {hideElem, showElem, toggleElem} from '../utils/dom.js';

const service = document.getElementById('service_type');
const user = document.getElementById('auth_username');
const pass = document.getElementById('auth_password');
const token = document.getElementById('auth_token');
const mirror = document.getElementById('mirror');
const lfs = document.getElementById('lfs');
const lfsSettings = document.getElementById('lfs_settings');
const lfsEndpoint = document.getElementById('lfs_endpoint');
const items = document.querySelectorAll('#migrate_items input[type=checkbox]');

export function initRepoMigration() {
  checkAuth();
  setLFSSettingsVisibility();

  user?.addEventListener('input', () => {checkItems(false)});
  pass?.addEventListener('input', () => {checkItems(false)});
  token?.addEventListener('input', () => {checkItems(true)});
  mirror?.addEventListener('change', () => {checkItems(true)});
  document.getElementById('lfs_settings_show')?.addEventListener('click', (e) => {
    e.preventDefault();
    e.stopPropagation();
    showElem(lfsEndpoint);
  });
  lfs?.addEventListener('change', setLFSSettingsVisibility);

  const cloneAddr = document.getElementById('clone_addr');
  cloneAddr?.addEventListener('change', () => {
    const repoName = document.getElementById('repo_name');
    if (cloneAddr.value && !repoName?.value) { // Only modify if repo_name input is blank
      repoName.value = cloneAddr.value.match(/^(.*\/)?((.+?)(\.git)?)$/)[3];
    }
  });
}

function checkAuth() {
  if (!service) return;
  const serviceType = Number(service.value);

  checkItems(serviceType !== 1);
}

function checkItems(tokenAuth) {
  let enableItems;
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
