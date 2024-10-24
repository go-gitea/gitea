export function initRequireActionsSelect() {
  const raselect = document.querySelector('add-require-actions-modal');
  if (!raselect) return;
  const checkboxes = document.querySelectorAll('.ui.radio.checkbox');
  for (const box of checkboxes) {
    box.addEventListener('change', function() {
      const hiddenInput = this.nextElementSibling;
      const isChecked = this.querySelector('input[type="radio"]').checked;
      hiddenInput.disabled = !isChecked;
      // Disable other hidden inputs
      for (const otherbox of checkboxes) {
        const otherHiddenInput = otherbox.nextElementSibling;
        if (otherbox !== box) {
          otherHiddenInput.disabled = isChecked;
        }
      }
    });
  }
}
