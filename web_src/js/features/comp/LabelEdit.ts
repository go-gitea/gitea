import {queryElems, toggleElem} from '../../utils/dom.ts';
import {showFomanticModal} from '../../modules/fomantic/modal.ts';
import {submitFormFetchAction} from '../../modules/fetch-action.ts';
import {registerGlobalInitFunc} from '../../modules/observer.ts';

function nameHasScope(name: string): boolean {
  return /.*[^/]\/[^/].*/.test(name);
}

function initLabelEditModal(elModal: HTMLElement) {
  const elLabelId = elModal.querySelector<HTMLInputElement>('input[name="id"]')!;
  const elNameInput = elModal.querySelector<HTMLInputElement>('.label-name-input')!;
  const elExclusiveField = elModal.querySelector('.label-exclusive-input-field')!;
  const elExclusiveInput = elModal.querySelector<HTMLInputElement>('.label-exclusive-input')!;
  const elExclusiveWarning = elModal.querySelector('.label-exclusive-warning')!;
  const elExclusiveOrderField = elModal.querySelector<HTMLInputElement>('.label-exclusive-order-input-field')!;
  const elExclusiveOrderInput = elModal.querySelector<HTMLInputElement>('.label-exclusive-order-input')!;
  const elIsArchivedField = elModal.querySelector('.label-is-archived-input-field')!;
  const elIsArchivedInput = elModal.querySelector<HTMLInputElement>('.label-is-archived-input')!;
  const elDescInput = elModal.querySelector<HTMLInputElement>('.label-desc-input')!;
  const elColorInput = elModal.querySelector<HTMLInputElement>('.color-picker-combo input')!;

  const syncModalUi = () => {
    const hasScope = nameHasScope(elNameInput.value);
    elExclusiveField.classList.toggle('disabled', !hasScope);
    const showExclusiveWarning = hasScope && elExclusiveInput.checked && elModal.hasAttribute('data-need-warn-exclusive');
    toggleElem(elExclusiveWarning, showExclusiveWarning);
    if (!hasScope) elExclusiveInput.checked = false;
    toggleElem(elExclusiveOrderField, elExclusiveInput.checked);

    if (parseInt(elExclusiveOrderInput.value) <= 0) {
      elExclusiveOrderInput.style.color = 'var(--color-placeholder-text) !important';
    } else {
      elExclusiveOrderInput.style.removeProperty('color');
    }
  };

  const showLabelEditModal = (btn:HTMLElement) => {
    // the "btn" should contain the label's attributes by its `data-label-xxx` attributes
    const form = elModal.querySelector<HTMLFormElement>('form')!;
    elLabelId.value = btn.getAttribute('data-label-id') || '';
    elNameInput.value = btn.getAttribute('data-label-name') || '';
    elExclusiveOrderInput.value = btn.getAttribute('data-label-exclusive-order') || '0';
    elIsArchivedInput.checked = btn.getAttribute('data-label-is-archived') === 'true';
    elExclusiveInput.checked = btn.getAttribute('data-label-exclusive') === 'true';
    elDescInput.value = btn.getAttribute('data-label-description') || '';
    elColorInput.value = btn.getAttribute('data-label-color') || '';
    elColorInput.dispatchEvent(new Event('input', {bubbles: true})); // trigger the color picker

    // if label id exists: "edit label" mode; otherwise: "new label" mode
    const isEdit = Boolean(elLabelId.value);

    // if a label was not exclusive but has issues, then it should warn user if it will become exclusive
    const numIssues = parseInt(btn.getAttribute('data-label-num-issues') || '0');
    elModal.toggleAttribute('data-need-warn-exclusive', !elExclusiveInput.checked && numIssues > 0);
    elModal.querySelector('.header')!.textContent = isEdit ? elModal.getAttribute('data-text-edit-label') : elModal.getAttribute('data-text-new-label');

    const curPageLink = elModal.getAttribute('data-current-page-link');
    form.action = isEdit ? `${curPageLink}/edit` : `${curPageLink}/new`;
    toggleElem(elIsArchivedField, isEdit);
    syncModalUi();
    showFomanticModal(elModal, {
      onApprove() {
        if (!form.checkValidity()) {
          form.reportValidity();
          return false;
        }
        submitFormFetchAction(form);
        return false;
      },
    });
  };

  elModal.addEventListener('input', () => syncModalUi());

  const elNewLabel = document.querySelector<HTMLElement>('.ui.button.new-label')!;
  elNewLabel.addEventListener('click', () => showLabelEditModal(elNewLabel));

  queryElems(document, '.edit-label-button', (btn) => btn.addEventListener('click', (e) => {
    e.preventDefault();
    showLabelEditModal(btn);
  }));
}

export function initCompLabelEdit() {
  registerGlobalInitFunc('initLabelEditModal', initLabelEditModal);
}
