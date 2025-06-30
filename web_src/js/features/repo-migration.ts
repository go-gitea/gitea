import {hideElem, showElem, toggleElem} from '../utils/dom.ts';
import {sanitizeRepoName} from './repo-common.ts';

const service = document.querySelector<HTMLInputElement>('#service_type');
const user = document.querySelector<HTMLInputElement>('#auth_username');
const pass = document.querySelector<HTMLInputElement>('#auth_password');
const token = document.querySelector<HTMLInputElement>('#auth_token');
const mirror = document.querySelector<HTMLInputElement>('#mirror');
const lfs = document.querySelector<HTMLInputElement>('#lfs');
const lfsSettings = document.querySelector<HTMLElement>('#lfs_settings');
const lfsEndpoint = document.querySelector<HTMLElement>('#lfs_endpoint');
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
      repoNameFromUrl = /^(.*\/)?((.+?)\/?)$/.exec(repoNameFromUrl)[3];
      repoNameFromUrl = repoNameFromUrl.split(/[?#]/)[0];
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
  let enableItems = false;
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
