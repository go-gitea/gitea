import {registerGlobalInitFunc} from '../modules/observer.ts';
import {toggleElem, toggleElemClass} from '../utils/dom.ts';

export function initActionsPermissionsForm(): void {
  registerGlobalInitFunc('initRepoActionsPermissionsForm', initRepoActionsPermissionsForm);
  registerGlobalInitFunc('initOwnerActionsPermissionsForm', initOwnerActionsPermissionsForm);
}

function initRepoActionsPermissionsForm(form: HTMLFormElement) {
  initActionsOverrideOwnerConfig(form);
  initActionsPermissionTable(form);
}

function initOwnerActionsPermissionsForm(form: HTMLFormElement) {
  initActionsPermissionTable(form);
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
