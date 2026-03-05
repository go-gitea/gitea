import {registerGlobalInitFunc} from '../modules/observer.ts';
import {queryElems, toggleElem, toggleElemClass} from '../utils/dom.ts';

export function initActionsPermissionsForm(): void {
  registerGlobalInitFunc('initRepoActionsPermissionsForm', initRepoActionsPermissionsForm);
  registerGlobalInitFunc('initOwnerActionsPermissionsForm', initOwnerActionsPermissionsForm);
}

function initRepoActionsPermissionsForm(form: HTMLFormElement) {
  initActionsOverrideOwnerConfig(form);
  initActionsPermissionTable(form);
}

function initOwnerActionsPermissionsForm(form: HTMLFormElement) {
  initActionsCrossRepoSetting(form);
  initActionsPermissionTable(form);
}

function initActionsCrossRepoSetting(form: HTMLFormElement) {
  // show or hide "selected repos only" section based on cross repo mode
  const selectedOnlySection = form.querySelector('.js-cross-repo-selected-only')!;
  function onCrossRepoModeChange(): void {
    const modeChecked = form.querySelector<HTMLInputElement>('input[name=cross_repo_mode]:checked');
    const isSelectedOnly = modeChecked?.value === 'selected';
    toggleElem(selectedOnlySection, isSelectedOnly);
  }
  onCrossRepoModeChange();
  queryElems(form, 'input[name=cross_repo_mode]', (el) => el.addEventListener('change', onCrossRepoModeChange));
}

function initActionsPermissionTable(form: HTMLFormElement) {
  // show or hide permissions table based on enable max permissions checkbox (aka: whether you use custom permissions or not)
  const permTable = form.querySelector<HTMLTableElement>('.js-permissions-table')!;
  const enableMaxCheckbox = form.querySelector<HTMLInputElement>('input[name=enable_max_permissions]')!;
  const onEnableMaxCheckboxChange = () => toggleElem(permTable, enableMaxCheckbox.checked);
  onEnableMaxCheckboxChange();
  enableMaxCheckbox.addEventListener('change', onEnableMaxCheckboxChange);
}

function initActionsOverrideOwnerConfig(form: HTMLFormElement) {
  // enable or disable repo token permissions config section based on override owner config checkbox
  const overrideOwnerConfig = form.querySelector<HTMLInputElement>('input[name=override_owner_config]')!;
  const repoTokenPermConfigSection = form.querySelector('.js-repo-token-permissions-config')!;
  const onOverrideOwnerConfigChange = () => toggleElemClass(repoTokenPermConfigSection, 'container-disabled', !overrideOwnerConfig.checked);
  onOverrideOwnerConfigChange();
  overrideOwnerConfig.addEventListener('change', onOverrideOwnerConfigChange);
}
