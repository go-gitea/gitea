import {setFileFolding} from './file-fold.js';

const viewedStyleClass = 'viewed-file-checked-checkbox';

// Initializes a listener for all children of the given html element
// (for example 'document' in the most basic case)
// to watch for changes of viewed-file checkboxes
export function initViewedCheckboxListenerFor(element) {
  for (const wholeCheckbox of element.querySelectorAll('.viewed-file-checkbox')) {
    // The checkbox consists of a div containing the real checkbox, and a label,
    // hence the actual checkbox first has to be found
    const checkbox = wholeCheckbox.querySelector('input[type=checkbox]');
    checkbox.addEventListener('change', function() {
      // Mark visually as viewed - will especially change the background of the whole block
      if (this.checked) {
        wholeCheckbox.classList.add(viewedStyleClass);
        window.config.pageData.numberOfViewedFiles++;
      } else {
        wholeCheckbox.classList.remove(viewedStyleClass);
        window.config.pageData.numberOfViewedFiles--;
      }

      // Update viewed-files summary
      document.getElementById('viewed-files-summary').setAttribute('value', window.config.pageData.numberOfViewedFiles);
      const summaryLabel = document.getElementById('viewed-files-summary-label');
      summaryLabel.innerHTML = summaryLabel.getAttribute('data-text-changed-template')
        .replace('$1', window.config.pageData.numberOfViewedFiles)
        .replace('$2', window.config.pageData.numberOfFiles);

      // Fold the file accordingly
      const parentBox = wholeCheckbox.closest('.diff-file-header');
      setFileFolding(parentBox.closest('.file-content'), parentBox.querySelector('.fold-file'), this.checked);
    });
  }
}
