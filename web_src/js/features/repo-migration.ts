import {hideElem, showElem, toggleElem} from '../utils/dom.ts';
import {sanitizeRepoName} from './repo-common.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';

export const initRepoMigration = () => registerGlobalInitFunc('initGitMigrateForm', (el: HTMLElement) => {
  const elForm = el.closest('form')!;
  const service = elForm.querySelector<HTMLInputElement>('input[name="service"]')!;
  const user = elForm.querySelector<HTMLInputElement>('input[name="auth_username"]');
  const pass = elForm.querySelector<HTMLInputElement>('input[name="auth_password"]');
  const token = elForm.querySelector<HTMLInputElement>('input[name="auth_token"]');
  const mirror = elForm.querySelector<HTMLInputElement>('input[name="mirror"]');
  const lfs = elForm.querySelector<HTMLInputElement>('input[name="lfs"]');
  const lfsSettings = elForm.querySelector<HTMLElement>('#lfs_settings');
  const lfsEndpoint = elForm.querySelector<HTMLElement>('#lfs_endpoint');
  const items = elForm.querySelectorAll<HTMLInputElement>('#migrate_items input[type=checkbox]');

  checkItems(Number(service.value) !== 1); // 1 = plain git service
  setLFSSettingsVisibility();

  user?.addEventListener('input', () => checkItems(false));
  pass?.addEventListener('input', () => checkItems(false));
  token?.addEventListener('input', () => checkItems(true));
  mirror?.addEventListener('change', () => checkItems(true));
  elForm.querySelector('#lfs_settings_show')?.addEventListener('click', (e) => {
    e.preventDefault();
    e.stopPropagation();
    showElem(lfsEndpoint!);
  });
  lfs?.addEventListener('change', setLFSSettingsVisibility);

  const elCloneAddr = elForm.querySelector<HTMLInputElement>('input[name="clone_addr"]');
  const elRepoName = elForm.querySelector<HTMLInputElement>('input[name="repo_name"]');
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
    toggleElem(lfsSettings!, visible);
    hideElem(lfsEndpoint!);
  }
});
