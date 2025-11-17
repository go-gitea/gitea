import {fomanticQuery} from '../modules/fomantic/base.ts';

export function initRepoDeleteFile() {
  const deleteButton = document.querySelector('#delete-file-button');
  const deleteModal = document.querySelector('#delete-file-modal');
  const deleteForm = document.querySelector<HTMLFormElement>('#delete-file-form');

  if (!deleteButton || !deleteModal || !deleteForm) {
    return;
  }

  deleteButton.addEventListener('click', (e) => {
    e.preventDefault();
    fomanticQuery(deleteModal).modal('show');
  });

  // Handle form submission
  deleteForm.addEventListener('submit', () => {
    fomanticQuery(deleteModal).modal('hide');
  });

  // Handle commit choice radio buttons
  const commitChoiceRadios = deleteForm.querySelectorAll<HTMLInputElement>('input[name="commit_choice"]');
  const newBranchNameContainer = deleteForm.querySelector('.quick-pull-branch-name');
  const newBranchNameInput = deleteForm.querySelector<HTMLInputElement>('input[name="new_branch_name"]');

  for (const radio of commitChoiceRadios) {
    radio.addEventListener('change', () => {
      if (radio.value === 'commit-to-new-branch') {
        newBranchNameContainer?.classList.remove('tw-hidden');
        if (newBranchNameInput) {
          newBranchNameInput.required = true;
        }
      } else {
        newBranchNameContainer?.classList.add('tw-hidden');
        if (newBranchNameInput) {
          newBranchNameInput.required = false;
        }
      }
    });
  }
}
