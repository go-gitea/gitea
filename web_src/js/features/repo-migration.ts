import {hideElem, showElem, toggleElem} from '../utils/dom.ts';
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

export function initOrgMigration() {
  const orgService = document.querySelector<HTMLInputElement>('#service');
  const orgToken = document.querySelector<HTMLInputElement>('#auth_token');
  const orgUser = document.querySelector<HTMLInputElement>('#auth_username');
  const orgPass = document.querySelector<HTMLInputElement>('#auth_password');
  if (!orgToken || !orgService) return;
  if (!document.querySelector('.page-content.organization.migrate')) return;

  const orgTokenField = document.querySelector<HTMLElement>('#auth_token_field');
  const orgUserField = document.querySelector<HTMLElement>('#auth_username_field');
  const orgPassField = document.querySelector<HTMLElement>('#auth_password_field');
  const orgItems = document.querySelectorAll<HTMLInputElement>('#migrate_items input[type=checkbox]');

  // Service types that support token auth (same as TokenAuth() in structs)
  // GithubService = 2, GiteaService = 3, GitlabService = 4
  const tokenAuthServices = [2, 3, 4];

  const checkOrgAuthFields = () => {
    const serviceType = Number(orgService.value);
    const useTokenAuth = tokenAuthServices.includes(serviceType);

    if (orgTokenField) toggleElem(orgTokenField, useTokenAuth);
    if (orgUserField) toggleElem(orgUserField, !useTokenAuth);
    if (orgPassField) toggleElem(orgPassField, !useTokenAuth);
  };

  const checkOrgItems = () => {
    const serviceType = Number(orgService.value);
    const useTokenAuth = tokenAuthServices.includes(serviceType);

    let enable: boolean;
    if (useTokenAuth) {
      enable = orgToken.value !== '';
    } else {
      enable = (orgUser?.value !== '') || (orgPass?.value !== '');
    }
    for (const item of orgItems) item.disabled = !enable;
  };

  checkOrgAuthFields();
  checkOrgItems();

  orgService.addEventListener('change', () => {
    checkOrgAuthFields();
    checkOrgItems();
  });
  orgToken.addEventListener('input', checkOrgItems);
  orgUser?.addEventListener('input', checkOrgItems);
  orgPass?.addEventListener('input', checkOrgItems);

  const orgLfs = document.querySelector<HTMLInputElement>('#lfs');
  const orgLfsSettings = document.querySelector<HTMLElement>('#lfs_settings');
  const orgLfsEndpoint = document.querySelector<HTMLElement>('#lfs_endpoint');
  if (orgLfs && orgLfsSettings && orgLfsEndpoint) {
    const setOrgLFSVisibility = () => {
      toggleElem(orgLfsSettings, orgLfs.checked);
      hideElem(orgLfsEndpoint);
    };
    setOrgLFSVisibility();
    orgLfs.addEventListener('change', setOrgLFSVisibility);
    document.querySelector('#lfs_settings_show')?.addEventListener('click', (e) => {
      e.preventDefault();
      e.stopPropagation();
      showElem(orgLfsEndpoint);
    });
  }
}

export function initRepoMigration() {
  if (!document.querySelector('.page-content.repository.migrate')) return;
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
