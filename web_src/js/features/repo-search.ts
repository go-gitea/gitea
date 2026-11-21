import {registerGlobalInitFunc} from '../modules/observer.ts';

export function initRepositorySearch() {
  registerGlobalInitFunc('initRepositorySearch', (form: HTMLFormElement) => {
    const radios = form.querySelectorAll<HTMLInputElement>('input[type="radio"]'); // bind directly, the filter popup leaves the form while open
    for (const radio of radios) radio.addEventListener('change', () => form.submit());
    form.querySelector('.repo-search-filter-reset')!.addEventListener('click', () => {
      for (const radio of radios) radio.checked = false;
      form.submit();
    });
  });
}
