import {setFileFolding} from './file-fold.js';

const {csrfToken, pageData} = window.config;
const prReview = pageData.prReview || {};
const viewedStyleClass = 'viewed-file-checked-form';
const viewedCheckboxSelector = '.viewed-file-form'; // Selector under which all "Viewed" checkbox forms can be found
const expandFilesBtnSelector = '#expand-files-btn';
const collapseFilesBtnSelector = '#collapse-files-btn';


// Refreshes the summary of viewed files if present
// The data used will be window.config.pageData.prReview.numberOf{Viewed}Files
function refreshViewedFilesSummary() {
  const viewedFilesProgress = document.getElementById('viewed-files-summary');
  viewedFilesProgress?.setAttribute('value', prReview.numberOfViewedFiles);
  const summaryLabel = document.getElementById('viewed-files-summary-label');
  if (summaryLabel) summaryLabel.innerHTML = summaryLabel.getAttribute('data-text-changed-template')
    .replace('%[1]d', prReview.numberOfViewedFiles)
    .replace('%[2]d', prReview.numberOfFiles);
}

// Explicitly recounts how many files the user has currently reviewed by counting the number of checked "viewed" checkboxes
// Additionally, the viewed files summary will be updated if it exists
export function countAndUpdateViewedFiles() {
  // The number of files is constant, but the number of viewed files can change because files can be loaded dynamically
  prReview.numberOfViewedFiles = document.querySelectorAll(`${viewedCheckboxSelector} > input[type=checkbox][checked]`).length;
  refreshViewedFilesSummary();
}

// Initializes a listener for all children of the given html element
// (for example 'document' in the most basic case)
// to watch for changes of viewed-file checkboxes
export function initViewedCheckboxListenerFor() {
  for (const form of document.querySelectorAll(`${viewedCheckboxSelector}:not([data-has-viewed-checkbox-listener="true"])`)) {
    // To prevent double addition of listeners
    form.setAttribute('data-has-viewed-checkbox-listener', true);

    // The checkbox consists of a div containing the real checkbox with its label and the CSRF token,
    // hence the actual checkbox first has to be found
    const checkbox = form.querySelector('input[type=checkbox]');
    checkbox.addEventListener('change', function() {
      // Mark the file as viewed visually - will especially change the background
      if (this.checked) {
        form.classList.add(viewedStyleClass);
        prReview.numberOfViewedFiles++;
      } else {
        form.classList.remove(viewedStyleClass);
        prReview.numberOfViewedFiles--;
      }

      // Update viewed-files summary and remove "has changed" label if present
      refreshViewedFilesSummary();
      const hasChangedLabel = form.parentNode.querySelector('.changed-since-last-review');
      hasChangedLabel?.parentNode.removeChild(hasChangedLabel);

      // Unfortunately, actual forms cause too many problems, hence another approach is needed
      const files = {};
      files[checkbox.getAttribute('name')] = this.checked;
      const data = {files};
      const headCommitSHA = form.getAttribute('data-headcommit');
      if (headCommitSHA) data.headCommitSHA = headCommitSHA;
      fetch(form.getAttribute('data-link'), {
        method: 'POST',
        headers: {'X-Csrf-Token': csrfToken},
        body: JSON.stringify(data),
      });

      // Fold the file accordingly
      const parentBox = form.closest('.diff-file-header');
      setFileFolding(parentBox.closest('.file-content'), parentBox.querySelector('.fold-file'), this.checked);
    });
  }
}

export function initExpandAndCollapseFilesButton() {
  // expand btn
  document.querySelector(expandFilesBtnSelector)?.addEventListener('click', () => {
    for (const box of document.querySelectorAll('.file-content[data-folded="true"]')) {
      setFileFolding(box, box.querySelector('.fold-file'), false);
    }
  });
  // collapse btn, need to exclude the div of “show more”
  document.querySelector(collapseFilesBtnSelector)?.addEventListener('click', () => {
    for (const box of document.querySelectorAll('.file-content:not([data-folded="true"])')) {
      if (box.getAttribute('id') === 'diff-incomplete') continue;
      setFileFolding(box, box.querySelector('.fold-file'), true);
    }
  });
}


