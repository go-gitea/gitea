function initOrgVisibilityChange() {
  const visibilityModal = document.querySelector('#change-visibility-org-modal');
  if (!visibilityModal) return;

  const visibilitySelect = visibilityModal.querySelectorAll<HTMLInputElement>("input[name='visibility']");
  if (!visibilitySelect) return;

  const currentValue = visibilityModal.querySelector<HTMLInputElement>('input[name="current_visibility"]').value;

  for (const radio of visibilitySelect) {
    radio.addEventListener('change', () => {
      const selectedValue = visibilityModal.querySelector<HTMLInputElement>("input[name='visibility']:checked").value;
      const btn = visibilityModal.querySelector<HTMLButtonElement>('#change-visibility-submit');
      if (selectedValue === currentValue) {
        btn.setAttribute('disabled', '');
      } else {
        btn.removeAttribute('disabled');
      }
    });
  }
}

export function initOrgSettings() {
  initOrgVisibilityChange();
}
