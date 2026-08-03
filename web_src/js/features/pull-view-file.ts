import {diffTreeStore, diffTreeStoreSetViewed} from '../modules/diff-file.ts';
import {setFileFolding} from './file-fold.ts';
import {POST} from '../modules/fetch.ts';
import {trString} from '../modules/i18n.ts';

const {pageData} = window.config;
// it is undefined on most pages, fortunately, when it is accessed by the related functions, it exists
const prReview = pageData.prReview!;
const viewedStyleClass = 'viewed-file-checked-form';
const expandFilesBtnSelector = '#expand-files-btn';
const collapseFilesBtnSelector = '#collapse-files-btn';

// Refreshes the summary of viewed files
// The data used will be window.config.pageData.prReview.numberOf{Viewed}Files
function refreshViewedFilesSummary() {
  const viewedFilesProgress = document.querySelector('#viewed-files-summary')!;
  viewedFilesProgress.setAttribute('value', String(prReview.numberOfViewedFiles));
  const summaryLabel = document.querySelector<HTMLElement>('#viewed-files-summary-label')!;
  const trText = summaryLabel.getAttribute('data-text-changed-template')!;
  summaryLabel.textContent = trString(trText, prReview.numberOfViewedFiles, prReview.numberOfFiles);
}

// Initializes a listener for viewed-file checkboxes
export function initDiffFileViewedForm(el: Element) {
  // The checkbox consists of a div containing the real checkbox with its label,
  // hence the actual checkbox first has to be found
  const checkbox = el.querySelector<HTMLInputElement>('input[type=checkbox]')!;
  checkbox.addEventListener('input', function() {
    // Mark the file as viewed visually - will especially change the background
    if (this.checked) {
      el.classList.add(viewedStyleClass);
      checkbox.setAttribute('checked', '');
      prReview.numberOfViewedFiles++;
    } else {
      el.classList.remove(viewedStyleClass);
      checkbox.removeAttribute('checked');
      prReview.numberOfViewedFiles--;
    }

    // Update viewed-files summary and remove "has changed" label if present
    refreshViewedFilesSummary();
    const hasChangedLabel = el.parentNode!.querySelector('.changed-since-last-review');
    hasChangedLabel?.remove();

    const fileName = checkbox.getAttribute('name')!;

    // check if the file is in our diffTreeStore and if we find it -> change the IsViewed status
    diffTreeStoreSetViewed(diffTreeStore(), fileName, this.checked);

    // Unfortunately, actual forms cause too many problems, hence another approach is needed
    const files: Record<string, boolean> = {};
    files[fileName] = this.checked;
    const data: Record<string, any> = {files};
    const headCommitSHA = el.getAttribute('data-headcommit');
    if (headCommitSHA) data.headCommitSHA = headCommitSHA;
    POST(el.getAttribute('data-link')!, {data});

    // Fold the file accordingly
    const parentBox = el.closest('.diff-file-header')!;
    setFileFolding(parentBox.closest('.file-content')!, parentBox.querySelector('.fold-file')!, this.checked);
  });
}

export function initExpandAndCollapseFilesButton() {
  // expand btn
  document.querySelector(expandFilesBtnSelector)?.addEventListener('click', () => {
    for (const box of document.querySelectorAll<HTMLElement>('.file-content[data-folded="true"]')) {
      setFileFolding(box, box.querySelector('.fold-file')!, false);
    }
  });
  // collapse btn, need to exclude the div of “show more”
  document.querySelector(collapseFilesBtnSelector)?.addEventListener('click', () => {
    for (const box of document.querySelectorAll<HTMLElement>('.file-content:not([data-folded="true"])')) {
      if (box.getAttribute('id') === 'diff-incomplete') continue;
      setFileFolding(box, box.querySelector('.fold-file')!, true);
    }
  });
}
