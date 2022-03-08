import {setFileFolding} from './file-fold.js';

const viewedStyleClass = 'viewed-file-checked-form';

function refreshViewedFilesSummary() {
  const viewedFilesMeter = document.getElementById('viewed-files-summary');
  if (viewedFilesMeter) viewedFilesMeter.setAttribute('value', window.config.pageData.numberOfViewedFiles);
  const summaryLabel = document.getElementById('viewed-files-summary-label');
  if (summaryLabel) summaryLabel.innerHTML = summaryLabel.getAttribute('data-text-changed-template')
    .replace('%[1]d', window.config.pageData.numberOfViewedFiles)
    .replace('%[2]d', window.config.pageData.numberOfFiles);
}

// Initializes a listener for all children of the given html element
// (for example 'document' in the most basic case)
// to watch for changes of viewed-file checkboxes
export function initViewedCheckboxListenerFor(element) {
  for (const form of element.querySelectorAll('.viewed-file-form:not([data-has-listener="true"])')) {
    // To prevent double addition of listeners
    form.setAttribute('data-has-listener', true);

    // The checkbox consists of a div containing the real checkbox with its label and the CSRF token,
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
      refreshViewedFilesSummary();

      // Unfortunately, using an actual form causes too many problems, hence we have to emulate the form
      const data = new FormData();
      data.append('_headCommitSHA', form.getAttribute('data-headcommit'));
      data.append('_csrf', form.querySelector('input[name=_csrf]').getAttribute('value'));
      data.append(checkbox.getAttribute('name'), this.checked);
      const request = new XMLHttpRequest();
      request.open('POST', form.getAttribute('data-link'));
      request.send(data);

      // Fold the file accordingly
      const parentBox = form.closest('.diff-file-header');
      setFileFolding(parentBox.closest('.file-content'), parentBox.querySelector('.fold-file'), this.checked);
    });
  }
}
