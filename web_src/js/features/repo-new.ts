import {createElementFromAttrs, hideElem, querySingleVisibleElem, showElem, toggleElem} from '../utils/dom.ts';
import {initDropdown} from '../modules/dropdown.ts';
import {GET} from '../modules/fetch.ts';
import {sanitizeRepoName} from './repo-common.ts';

const {appSubUrl} = window.config;

function initRepoNewTemplateSearch(form: HTMLFormElement) {
  const elSubmitButton = querySingleVisibleElem<HTMLButtonElement>(form, '.ui.primary.button')!;
  const elCreateRepoErrorMessage = form.querySelector('#create-repo-error-message')!;
  const inputRepoTemplate = form.querySelector<HTMLInputElement>('#repo_template')!;
  const elTemplateUnits = form.querySelector('#template_units')!;
  const elNonTemplate = form.querySelector('#non_template')!;
  const checkTemplate = function () {
    const hasSelectedTemplate = inputRepoTemplate.value !== '' && inputRepoTemplate.value !== '0';
    toggleElem(elTemplateUnits, hasSelectedTemplate);
    toggleElem(elNonTemplate, !hasSelectedTemplate);
  };
  inputRepoTemplate.addEventListener('change', checkTemplate);
  checkTemplate();

  const repoOwnerDropdown = initDropdown(form.querySelector<HTMLElement>('#repo_owner_dropdown')!);
  const repoTemplateDropdown = initDropdown(form.querySelector<HTMLElement>('#repo_template_search')!, {
    async loadItems(query, signal) {
      const params = new URLSearchParams({q: query, template: 'true', priority_owner_id: repoOwnerDropdown.value});
      const response = await GET(`${appSubUrl}/repo/search?${params}`, {signal});
      const {data} = await response.json() as {data: Array<{repository: {full_name: string, id: number}}>};
      return data.map(({repository}) => createElementFromAttrs('div', {
        class: 'dropdown-item',
        'data-value': String(repository.id),
      }, repository.full_name));
    },
  });
  const onChangeOwner = function () {
    const ownerItem = repoOwnerDropdown.findItem(repoOwnerDropdown.value);
    elCreateRepoErrorMessage.textContent = ownerItem?.getAttribute('data-create-repo-disallowed-prompt') ?? '';
    const hasError = Boolean(elCreateRepoErrorMessage.textContent);
    toggleElem(elCreateRepoErrorMessage, hasError);
    elSubmitButton.disabled = hasError;
    if (repoTemplateDropdown.isShown()) repoTemplateDropdown.reload();
  };
  repoOwnerDropdown.valueInput!.addEventListener('change', onChangeOwner);
  onChangeOwner();
}

export function initRepoNew(form: HTMLFormElement) {
  const inputGitIgnores = form.querySelector<HTMLInputElement>('input[name="gitignores"]')!;
  const inputLicense = form.querySelector<HTMLInputElement>('input[name="license"]')!;
  const inputAutoInit = form.querySelector<HTMLInputElement>('input[name="auto_init"]')!;
  const updateUiAutoInit = () => {
    inputAutoInit.checked = Boolean(inputGitIgnores.value || inputLicense.value);
  };
  inputGitIgnores.addEventListener('change', updateUiAutoInit);
  inputLicense.addEventListener('change', updateUiAutoInit);
  updateUiAutoInit();

  const inputRepoName = form.querySelector<HTMLInputElement>('input[name="repo_name"]')!;
  const inputPrivate = form.querySelector<HTMLInputElement>('input[name="private"]')!;
  const updateUiRepoName = () => {
    const helps = form.querySelectorAll(`.help[data-help-for-repo-name]`);
    hideElem(helps);
    let help = form.querySelector(`.help[data-help-for-repo-name="${CSS.escape(inputRepoName.value)}"]`);
    if (!help) help = form.querySelector(`.help[data-help-for-repo-name=""]`)!;
    showElem(help);
    const repoNamePreferPrivate: Record<string, boolean> = {'.profile': false, '.profile-private': true};
    const preferPrivate = repoNamePreferPrivate[inputRepoName.value];
    // inputPrivate might be disabled because site admin "force private"
    if (preferPrivate !== undefined && !inputPrivate.closest('.disabled, [disabled]')) {
      inputPrivate.checked = preferPrivate;
    }
  };
  inputRepoName.addEventListener('input', updateUiRepoName);
  inputRepoName.addEventListener('change', () => {
    inputRepoName.value = sanitizeRepoName(inputRepoName.value);
    updateUiRepoName();
  });
  updateUiRepoName();

  initRepoNewTemplateSearch(form);
}
