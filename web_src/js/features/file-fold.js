import {svg} from '../svg.js';

// Hides the file if newFold is true, and shows it otherwise. The actual hiding is performed using CSS.
//
// The fold arrow is the icon displayed on the upper left of the file box, especially intended for components having the 'fold-file' class.
// The file content box is the box that should be hidden or shown, especially intended for components having the 'file-content' class.
//
export function setFileFolding(fileContentBox, foldArrow, newFold) {
  foldArrow.innerHTML = svg(`octicon-chevron-${newFold ? 'right' : 'down'}`, 18);
  fileContentBox.setAttribute('data-folded', newFold);
}

// Like `setFileFolding`, except that it automatically inverts the current file folding state.
export function invertFileFolding(fileContentBox, foldArrow) {
  const diffFileHeader = fileContentBox.querySelector('.diff-file-header');
  const isFolded = fileContentBox.getAttribute('data-folded');
  setFileFolding(fileContentBox, foldArrow, isFolded !== 'true');
  if (isFolded === 'false' && window.scrollY > diffFileHeader.offsetTop) {
    const nextDiffBox = fileContentBox.nextElementSibling;
    if (nextDiffBox) {
      window.scrollTo({
        top: nextDiffBox.offsetTop - document.querySelector('.diff-detail-box').offsetHeight,
        behavior: 'instant'
      });
    }
  }
}

