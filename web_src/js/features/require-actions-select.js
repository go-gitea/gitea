export function initRequireActionsSelect() {
  const raselect = document.getElementById('add-require-actions-modal');
  if (!raselect) return;
  const checkboxes = document.querySelectorAll('.ui.radio.checkbox');
  checkboxes.forEach(function(checkbox) {
    checkbox.addEventListener('change', function() {
      var hiddenInput = this.nextElementSibling;
      var isChecked = this.querySelector('input[type="radio"]').checked;
      hiddenInput.disabled = !isChecked;
      // Disable other hidden inputs
      checkboxes.forEach(function(otherCheckbox) {
        var otherHiddenInput = otherCheckbox.nextElementSibling;
        if (otherCheckbox !== checkbox) {
          otherHiddenInput.disabled = isChecked;
        }
      });
    });
  });
}
