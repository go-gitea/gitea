import {hideElem, showElem, toggleElem} from '../utils/dom.ts';
import {htmlEscape} from 'escape-goat';
import {fomanticQuery} from '../modules/fomantic/base.ts';

const {appSubUrl} = window.config;

function initRepoNewTemplateSearch(form: HTMLFormElement) {
  const inputRepoOwnerUid = form.querySelector<HTMLInputElement>('#uid');
  const elRepoTemplateDropdown = form.querySelector<HTMLInputElement>('#repo_template_search');
  const inputRepoTemplate = form.querySelector<HTMLInputElement>('#repo_template');
  const elTemplateUnits = form.querySelector('#template_units');
  const elNonTemplate = form.querySelector('#non_template');
  const checkTemplate = function () {
    const hasSelectedTemplate = inputRepoTemplate.value !== '' && inputRepoTemplate.value !== '0';
    toggleElem(elTemplateUnits, hasSelectedTemplate);
    toggleElem(elNonTemplate, !hasSelectedTemplate);
  };
  inputRepoTemplate.addEventListener('change', checkTemplate);
  checkTemplate();

  const $dropdown = fomanticQuery(elRepoTemplateDropdown);
  const onChangeOwner = function () {
    $dropdown.dropdown('setting', {
      apiSettings: {
        url: `${appSubUrl}/repo/search?q={query}&template=true&priority_owner_id=${inputRepoOwnerUid.value}`,
        onResponse(response) {
          const results = [];
          results.push({name: '', value: ''}); // empty item means not using template
          for (const tmplRepo of response.data) {
            results.push({
              name: htmlEscape(tmplRepo.repository.full_name),
              value: String(tmplRepo.repository.id),
            });
          }
          $dropdown.fomanticExt.onResponseKeepSelectedItem($dropdown, inputRepoTemplate.value);
          return {results};
        },
        cache: false,
      },
    });
  };
  inputRepoOwnerUid.addEventListener('change', onChangeOwner);
  onChangeOwner();
}

export function initRepoNew() {
  const pageContent = document.querySelector('.page-content.repository.new-repo');
  if (!pageContent) return;

  const form = document.querySelector<HTMLFormElement>('.new-repo-form');
  const inputGitIgnores = form.querySelector<HTMLInputElement>('input[name="gitignores"]');
  const inputLicense = form.querySelector<HTMLInputElement>('input[name="license"]');
  const inputAutoInit = form.querySelector<HTMLInputElement>('input[name="auto_init"]');
  const updateUiAutoInit = () => {
    inputAutoInit.checked = Boolean(inputGitIgnores.value || inputLicense.value);
  };
  inputGitIgnores.addEventListener('change', updateUiAutoInit);
  inputLicense.addEventListener('change', updateUiAutoInit);
  updateUiAutoInit();

  const inputRepoName = form.querySelector<HTMLInputElement>('input[name="repo_name"]');
  const inputPrivate = form.querySelector<HTMLInputElement>('input[name="private"]');
  const updateUiRepoName = () => {
    const helps = form.querySelectorAll(`.help[data-help-for-repo-name]`);
    hideElem(helps);
    let help = form.querySelector(`.help[data-help-for-repo-name="${CSS.escape(inputRepoName.value)}"]`);
    if (!help) help = form.querySelector(`.help[data-help-for-repo-name=""]`);
    showElem(help);
    const repoNamePreferPrivate = {'.profile': false, '.profile-private': true};
    const preferPrivate = repoNamePreferPrivate[inputRepoName.value];
    // inputPrivate might be disabled because site admin "force private"
    if (preferPrivate !== undefined && !inputPrivate.closest('.disabled, [disabled]')) {
      inputPrivate.checked = preferPrivate;
    }
  };
  inputRepoName.addEventListener('input', updateUiRepoName);
  updateUiRepoName();

  initRepoNewTemplateSearch(form);
}
