export function initNewProjectFormLabel() {
  if (!document.querySelectorAll('#new-project-column-item form input#new_project_column_project_label').length) return;

  const labels = document.querySelectorAll('.labels-for-project-creation');
  const selectedLabel = document.querySelector('input#new_project_column_project_label');
  const selectedLabelId = document.querySelector('input#new_project_column_project_label_id');

  if (!labels || !selectedLabel || !selectedLabelId) return;

  for (const l of labels) {
    l.addEventListener('click', () => {
      selectedLabel.value = l.textContent;
      selectedLabelId.value = l.dataset.labelId;
    });
  }
}
