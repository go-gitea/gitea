import {hideElem, showElem} from '../utils/dom.ts';

export function initRepoNew() {
  const pageContent = document.querySelector('.page-content.repository.new-repo');
  if (!pageContent) return;

  const form = document.querySelector('.new-repo-form');
  const inputGitIgnores = form.querySelector<HTMLInputElement>('input[name="gitignores"]');
  const inputLicense = form.querySelector<HTMLInputElement>('input[name="license"]');
  const inputAutoInit = form.querySelector<HTMLInputElement>('input[name="auto_init"]');
  const updateUiAutoInit = () => {
    inputAutoInit.checked = Boolean(inputGitIgnores.value || inputLicense.value);
  };
  form.addEventListener('change', updateUiAutoInit);
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
}
