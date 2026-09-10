import {registerGlobalInitFunc} from '../modules/observer.ts';
import {toggleElem, toggleElemClass} from '../utils/dom.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';

const {appSubUrl} = window.config;

type RepoSearchResponse = {data: Array<{repository: {id: number; full_name: string}}>};

export function initActionsSettings(): void {
  registerGlobalInitFunc('initRepoActionsPermissionsForm', initRepoActionsPermissionsForm);
  registerGlobalInitFunc('initOwnerActionsPermissionsForm', initOwnerActionsPermissionsForm);
  registerGlobalInitFunc('initRunnerAccessInput', initRunnerAccessInput);
  registerGlobalInitFunc('initRunnerGroupMembersInput', initRunnerGroupMembersInput);
}

function initRunnerAccessInput(el: HTMLElement) {
  const uid = el.getAttribute('data-uid')!;
  fomanticQuery(el).dropdown({
    preserveHTML: false,
    labelHref: (_value: string, text: string) => `${appSubUrl}/${text}`,
    apiSettings: {
      url: `${appSubUrl}/repo/search?q={query}&uid=${uid}&exclusive=${uid !== '0'}`,
      throttle: 500,
      onResponse: (res: RepoSearchResponse) => ({success: true, results: res.data.map((item) => ({value: String(item.repository.id), name: item.repository.full_name}))}),
    },
  });
}

function initRunnerGroupMembersInput(el: HTMLElement) {
  const runnerLink = el.getAttribute('data-runner-link')!;
  fomanticQuery(el).dropdown({preserveHTML: false, labelHref: (value: string) => `${runnerLink}/${value}`});
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
