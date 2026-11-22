import {registerGlobalInitFunc} from '../modules/observer.ts';

export function initDisclosureCreditsEditor() {
  registerGlobalInitFunc('initDisclosureCreditsEditor', (el: HTMLElement) => {
    const elRows = el.querySelector<HTMLElement>('#disclosure-credit-rows')!;
    const elTemplate = el.querySelector<HTMLTemplateElement>('#disclosure-credit-row-template')!;
    const elAdd = el.querySelector<HTMLButtonElement>('#disclosure-credit-add')!;

    // rows are named credit_*_<i> by their position, so the server can read them contiguously
    const renumber = () => {
      elRows.querySelectorAll<HTMLElement>('.disclosure-credit-row').forEach((row, i) => {
        row.querySelectorAll<HTMLElement>('[name]').forEach((input) => {
          input.setAttribute('name', input.getAttribute('name')!.replace(/_(\d+|__INDEX__)$/, () => `_${i}`));
        });
      });
    };

    // the URL field only applies to external people
    const syncUrlField = (row: HTMLElement) => {
      const isExternal = row.querySelector<HTMLSelectElement>('.disclosure-credit-type')!.value === 'external';
      row.querySelector<HTMLInputElement>('.disclosure-credit-url')!.disabled = !isExternal;
    };

    elAdd.addEventListener('click', () => {
      elRows.append(elTemplate.content.cloneNode(true));
      renumber();
      elRows.lastElementChild!.querySelector<HTMLElement>('input')!.focus();
    });

    elRows.addEventListener('click', (e) => {
      const removeButton = (e.target as HTMLElement).closest('.disclosure-credit-remove');
      if (!removeButton) return;
      removeButton.closest('.disclosure-credit-row')!.remove();
      renumber();
    });

    elRows.addEventListener('change', (e) => {
      if ((e.target as HTMLElement).classList.contains('disclosure-credit-type')) {
        syncUrlField((e.target as HTMLElement).closest('.disclosure-credit-row')!);
      }
    });

    elRows.querySelectorAll<HTMLElement>('.disclosure-credit-row').forEach(syncUrlField);
  });
}
