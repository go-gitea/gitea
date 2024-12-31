import {svg} from '../svg.ts';

// Hides the file if newFold is true, and shows it otherwise. The actual hiding is performed using CSS.
//
// The fold arrow is the icon displayed on the upper left of the file box, especially intended for components having the 'fold-file' class.
// The file content box is the box that should be hidden or shown, especially intended for components having the 'file-content' class.
//
export function setFileFolding(fileContentBox, foldArrow, newFold) {
  foldArrow.innerHTML = svg(`octicon-chevron-${newFold ? 'right' : 'down'}`, 18);
  fileContentBox.setAttribute('data-folded', newFold);
  if (newFold && fileContentBox.getBoundingClientRect().top < 0) {
    fileContentBox.scrollIntoView();
  }
}

// Like `setFileFolding`, except that it automatically inverts the current file folding state.
export function invertFileFolding(fileContentBox, foldArrow) {
  setFileFolding(fileContentBox, foldArrow, fileContentBox.getAttribute('data-folded') !== 'true');
}
