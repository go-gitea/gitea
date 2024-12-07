import {toggleElem} from '../../utils/dom.ts';
import {fomanticQuery} from '../../modules/fomantic/base.ts';

function nameHasScope(name: string): boolean {
  return /.*[^/]\/[^/].*/.test(name);
}

export function initCompLabelEdit(pageSelector: string) {
  const pageContent = document.querySelector<HTMLElement>(pageSelector);
  if (!pageContent) return;

  const elModal = pageContent.querySelector<HTMLElement>('#issue-label-edit-modal');
  if (!elModal) return;

  const elLabelId = elModal.querySelector<HTMLInputElement>('input[name="id"]');
  const elNameInput = elModal.querySelector<HTMLInputElement>('.label-name-input');
  const elExclusiveField = elModal.querySelector('.label-exclusive-input-field');
  const elExclusiveInput = elModal.querySelector<HTMLInputElement>('.label-exclusive-input');
  const elExclusiveWarning = elModal.querySelector('.label-exclusive-warning');
  const elIsArchivedField = elModal.querySelector('.label-is-archived-input-field');
  const elIsArchivedInput = elModal.querySelector<HTMLInputElement>('.label-is-archived-input');
  const elDescInput = elModal.querySelector<HTMLInputElement>('.label-desc-input');
  const elColorInput = elModal.querySelector<HTMLInputElement>('.js-color-picker-input input');

  const syncModalUi = () => {
    const hasScope = nameHasScope(elNameInput.value);
    elExclusiveField.classList.toggle('disabled', !hasScope);
    const showExclusiveWarning = hasScope && elExclusiveInput.checked && elModal.hasAttribute('data-need-warn-exclusive');
    toggleElem(elExclusiveWarning, showExclusiveWarning);
    if (!hasScope) elExclusiveInput.checked = false;
  };

  const showLabelEditModal = (btn:HTMLElement) => {
    const form = elModal.querySelector<HTMLFormElement>('form');
    elLabelId.value = btn.getAttribute('data-label-id') || '';
    elNameInput.value = btn.getAttribute('data-label-name') || '';
    elIsArchivedInput.checked = btn.getAttribute('data-label-is-archived') === 'true';
    elExclusiveInput.checked = btn.getAttribute('data-label-exclusive') === 'true';
    elDescInput.value = btn.getAttribute('data-label-description') || '';
    elColorInput.value = btn.getAttribute('data-label-color') || '';
    elColorInput.dispatchEvent(new Event('input', {bubbles: true})); // trigger the color picker

    const isEdit = Boolean(elLabelId.value);

    // if a label was not exclusive but has issues, then it should warn user if it will become exclusive
    const numIssues = parseInt(btn.getAttribute('data-label-num-issues') || '0');
    elModal.toggleAttribute('data-need-warn-exclusive', !elExclusiveInput.checked && numIssues > 0);
    elModal.querySelector('.header').textContent = isEdit ? elModal.getAttribute('data-text-edit-label') : elModal.getAttribute('data-text-new-label');

    const repoLink = elModal.getAttribute('data-repo-link');
    form.action = isEdit ? `${repoLink}/edit` : `${repoLink}/new`;
    toggleElem(elIsArchivedField, isEdit);
    syncModalUi();
    fomanticQuery(elModal).modal({
      onApprove() {
        if (!form.checkValidity()) {
          form.reportValidity();
          return false;
        }
        form.submit();
      },
    }).modal('show');
  };

  elModal.addEventListener('input', () => syncModalUi());

  const elNewLabel = pageContent.querySelector<HTMLElement>('.ui.button.new-label');
  elNewLabel?.addEventListener('click', () => showLabelEditModal(elNewLabel));

  const elEditLabelButtons = pageContent.querySelectorAll<HTMLElement>('.edit-label-button');
  for (const btn of elEditLabelButtons) {
    btn.addEventListener('click', (e) => {
      e.preventDefault();
      showLabelEditModal(btn);
    });
  }
}
