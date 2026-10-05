import {registerGlobalInitFunc} from '../modules/observer.ts';
import {addDelegatedEventListener, queryElems} from '../utils/dom.ts';

export function initRepositorySearch() {
  registerGlobalInitFunc('initRepositorySearch', (form: HTMLFormElement) => {
    addDelegatedEventListener(form, 'change', 'input[type="radio"]', () => form.submit());
    addDelegatedEventListener(form, 'change', 'select', () => form.submit());
    form.querySelector('.repo-search-filter-reset')!.addEventListener('click', () => {
      queryElems(form, 'input[type="radio"]', (el: HTMLInputElement) => el.checked = false);
      queryElems(form, 'select', (el: HTMLSelectElement) => el.value = '');
      form.submit();
    });
  });
}

export function initExploreOrganizationSearch() {
  registerGlobalInitFunc('initExploreOrganizationSearch', (form: HTMLFormElement) => {
    form.querySelector<HTMLSelectElement>('select[name="badge"]')!.addEventListener('change', () => form.submit());
  });
}
