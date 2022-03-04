import {setFileFolding} from './file-fold.js';

const viewedStyleClass = 'viewed-file-checked-form';

// Initializes a listener for all children of the given html element
// (for example 'document' in the most basic case)
// to watch for changes of viewed-file checkboxes
export function initViewedCheckboxListenerFor(element) {
  for (const form of element.querySelectorAll('.viewed-file-form')) {
    // The checkbox consists of a form containing the real checkbox and a label,
    // hence the actual checkbox first has to be found
    const checkbox = form.querySelector('input[type=checkbox]');
    checkbox.addEventListener('change', function() {
      // Mark the file as viewed visually - will especially change the background
      if (this.checked) {
        form.classList.add(viewedStyleClass);
        window.config.pageData.numberOfViewedFiles++;
      } else {
        form.classList.remove(viewedStyleClass);
        window.config.pageData.numberOfViewedFiles--;
      }

      // Update viewed-files summary
      const viewedFilesMeter = document.getElementById('viewed-files-summary');
      if(viewedFilesMeter)
        viewedFilesMeter.setAttribute('value', window.config.pageData.numberOfViewedFiles);
      const summaryLabel = document.getElementById('viewed-files-summary-label');
      if(summaryLabel)
        summaryLabel.innerHTML = summaryLabel.getAttribute('data-text-changed-template')
          .replace('%[1]d', window.config.pageData.numberOfViewedFiles)
          .replace('%[2]d', window.config.pageData.numberOfFiles);

      // Unfortunately, form.submit() would attempt to redirect, so we have to workaround that
      // Because unchecked checkboxes are also not sent, we have to unset the value and use the fallback hidden input as sent value
      const previousCheckedState = this.checked;
      this.checked = false;
      this.previousElementSibling.setAttribute('value', previousCheckedState);
      const request = new XMLHttpRequest();
      request.open('POST', form.getAttribute('action'));
      request.send(new FormData(form));
      this.checked = previousCheckedState;

      // Fold the file accordingly
      const parentBox = form.closest('.diff-file-header');
      setFileFolding(parentBox.closest('.file-content'), parentBox.querySelector('.fold-file'), this.checked);
    });
  }
}
