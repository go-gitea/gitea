import {hideElem, showElem, toggleElem} from '../utils/dom.js';

const service = document.querySelector('#service_type');
const user = document.querySelector('#auth_username');
const pass = document.querySelector('#auth_password');
const token = document.querySelector('#auth_token');
const mirror = document.querySelector('#mirror');
const lfs = document.querySelector('#lfs');
const lfsSettings = document.querySelector('#lfs_settings');
const lfsEndpoint = document.querySelector('#lfs_endpoint');
const items = document.querySelectorAll('#migrate_items input[type=checkbox]');

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

  const cloneAddr = document.querySelector('#clone_addr');
  cloneAddr?.addEventListener('change', () => {
    const repoName = document.querySelector('#repo_name');
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
