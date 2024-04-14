import $ from 'jquery';

function isExclusiveScopeName(name) {
  return /.*[^/]\/[^/].*/.test(name);
}

function updateExclusiveLabelEdit(form) {
  const nameInput = document.querySelector(`${form} .label-name-input`);
  const exclusiveField = document.querySelector(`${form} .label-exclusive-input-field`);
  const exclusiveCheckbox = document.querySelector(`${form} .label-exclusive-input`);
  const exclusiveWarning = document.querySelector(`${form} .label-exclusive-warning`);

  if (isExclusiveScopeName(nameInput.value)) {
    exclusiveField?.classList.remove('muted');
    exclusiveField?.removeAttribute('aria-disabled');
    if (exclusiveCheckbox.checked && exclusiveCheckbox.getAttribute('data-exclusive-warn')) {
      exclusiveWarning?.classList.remove('tw-hidden');
    } else {
      exclusiveWarning?.classList.add('tw-hidden');
    }
  } else {
    exclusiveField?.classList.add('muted');
    exclusiveField?.setAttribute('aria-disabled', 'true');
    exclusiveWarning?.classList.add('tw-hidden');
  }
}

export function initCompLabelEdit(selector) {
  if (!$(selector).length) return;

  // Create label
  $('.new-label.button').on('click', () => {
    updateExclusiveLabelEdit('.new-label');
    $('.new-label.modal').modal({
      onApprove() {
        const form = document.querySelector('.new-label.form');
        if (!form.checkValidity()) {
          form.reportValidity();
          return false;
        }
        $('.new-label.form').trigger('submit');
      },
    }).modal('show');
    return false;
  });

  // Edit label
  $('.edit-label-button').on('click', function () {
    $('#label-modal-id').val($(this).data('id'));

    const $nameInput = $('.edit-label .label-name-input');
    $nameInput.val($(this).data('title'));

    const $isArchivedCheckbox = $('.edit-label .label-is-archived-input');
    $isArchivedCheckbox[0].checked = this.hasAttribute('data-is-archived');

    const $exclusiveCheckbox = $('.edit-label .label-exclusive-input');
    $exclusiveCheckbox[0].checked = this.hasAttribute('data-exclusive');
    // Warn when label was previously not exclusive and used in issues
    $exclusiveCheckbox.data('exclusive-warn',
      $(this).data('num-issues') > 0 &&
      (!this.hasAttribute('data-exclusive') || !isExclusiveScopeName($nameInput.val())));
    updateExclusiveLabelEdit('.edit-label');

    $('.edit-label .label-desc-input').val(this.getAttribute('data-description'));

    const colorInput = document.querySelector('.edit-label .js-color-picker-input input');
    colorInput.value = this.getAttribute('data-color');
    colorInput.dispatchEvent(new Event('input', {bubbles: true}));

    $('.edit-label.modal').modal({
      onApprove() {
        const form = document.querySelector('.edit-label.form');
        if (!form.checkValidity()) {
          form.reportValidity();
          return false;
        }
        $('.edit-label.form').trigger('submit');
      },
    }).modal('show');
    return false;
  });

  $('.new-label .label-name-input').on('input', () => {
    updateExclusiveLabelEdit('.new-label');
  });
  $('.new-label .label-exclusive-input').on('change', () => {
    updateExclusiveLabelEdit('.new-label');
  });
  $('.edit-label .label-name-input').on('input', () => {
    updateExclusiveLabelEdit('.edit-label');
  });
  $('.edit-label .label-exclusive-input').on('change', () => {
    updateExclusiveLabelEdit('.edit-label');
  });
}
