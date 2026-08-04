import {registerGlobalInitFunc} from '../modules/observer.ts';
import {addDelegatedEventListener, queryElems} from '../utils/dom.ts';

export function initRepositorySearch() {
  registerGlobalInitFunc('initRepositorySearch', (form: HTMLFormElement) => {
    addDelegatedEventListener(form, 'change', 'input[type="radio"]', () => form.submit());
    form.querySelector('.repo-search-filter-reset')!.addEventListener('click', () => {
      queryElems(form, 'input[type="radio"]', (el: HTMLInputElement) => el.checked = false);
      form.submit();
    });
  });
}
