import {fomanticQuery} from '../modules/fomantic/base.ts';

export function initRepoEditorCommit() {
  const commitButton = document.querySelector('#commit-changes-button');
  const commitModal = document.querySelector('#commit-changes-modal');
  const modalCommitButton = document.querySelector<HTMLButtonElement>('#commit-button');

  if (!commitButton || !commitModal) return;

  const elForm = document.querySelector<HTMLFormElement>('.repository.editor .edit.form');
  const dirtyFileClass = 'dirty-file';

  // Sync the top commit button state with the modal commit button state
  const syncTopCommitButtonState = () => {
    // Check if form has changes (using the same dirty class from repo-editor.ts)
    const hasChanges = elForm?.classList.contains(dirtyFileClass);

    // Also check the modal commit button state as fallback
    const modalButtonDisabled = modalCommitButton?.disabled;

    if (hasChanges || !modalButtonDisabled) {
      commitButton.classList.remove('disabled');
    } else {
      commitButton.classList.add('disabled');
    }
  };

  // For upload page - enable button when files are added
  const dropzone = document.querySelector('.dropzone');
  if (dropzone) {
    const observer = new MutationObserver(() => {
      const filesContainer = dropzone.querySelector('.files');
      const hasFiles = filesContainer && filesContainer.children.length > 0;

      if (hasFiles) {
        commitButton.classList.remove('disabled');
      } else {
        commitButton.classList.add('disabled');
      }
    });

    const filesContainer = dropzone.querySelector('.files');
    if (filesContainer) {
      observer.observe(filesContainer, {childList: true});
    }
  }

  // Watch for changes in the form's dirty state
  if (elForm) {
    const observer = new MutationObserver(syncTopCommitButtonState);
    observer.observe(elForm, {attributes: true, attributeFilter: ['class']});

    // Initial sync
    syncTopCommitButtonState();
  }

  // Also sync when modal commit button state changes
  if (modalCommitButton) {
    const observer = new MutationObserver(syncTopCommitButtonState);
    observer.observe(modalCommitButton, {attributes: true, attributeFilter: ['disabled']});
  }

  const commitFormFields = document.querySelector<HTMLElement>('#commit-form-fields');

  commitButton.addEventListener('click', (e) => {
    e.preventDefault();
    if (!commitButton.classList.contains('disabled')) {
      // Show the commit form fields (they stay in the form, just become visible)
      if (commitFormFields) {
        commitFormFields.style.display = 'block';
        // Position it inside the modal using CSS
        commitFormFields.classList.add('commit-form-in-modal');
      }
      fomanticQuery(commitModal).modal('show');
    }
  });

  // When modal closes, hide the form fields again
  fomanticQuery(commitModal).modal({
    onHidden: () => {
      if (commitFormFields) {
        commitFormFields.style.display = 'none';
        commitFormFields.classList.remove('commit-form-in-modal');
      }
    },
  });

  // Handle close button click
  const closeButton = document.querySelector('#commit-modal-close-btn');
  if (closeButton) {
    closeButton.addEventListener('click', () => {
      fomanticQuery(commitModal).modal('hide');
    });
  }

  // Handle form submission - close modal after submit
  if (elForm) {
    elForm.addEventListener('submit', () => {
      fomanticQuery(commitModal).modal('hide');
    });
  }
}
