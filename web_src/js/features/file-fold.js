import {svg} from '../svg.js';


// Hides the file if newFold is true, and shows it otherwise. The actual hiding is performed using CSS.
//
// The fold arrow is the icon displayed on the upper left of the file box, especially intended for components having the 'fold-file' class.
// The file content box is the box that should be hidden or shown, especially intended for components having the 'file-content' class.
//
function setFileFolding(fileContentBox, foldArrow, newFold) {
  foldArrow.innerHTML = svg(`octicon-chevron-${newFold ? 'right' : 'down'}`, 18);
  fileContentBox.setAttribute('data-folded', String(newFold));
}

// Like `setFileFolding`, except that it automatically inverts the current file folding state.
export function invertFileFolding(fileContentBox, foldArrow) {
  setFileFolding(fileContentBox, foldArrow, fileContentBox.getAttribute('data-folded') !== 'true');
}

// Folds every file stored in `window.config.pageData.viewedFiles` according to the value that is persisted there.
//
// All other files not present there are simply ignored.
export function resetFileFolding() {
  const viewedFiles = window.config.pageData.viewedFiles;
  if (!viewedFiles) return;

  const filenameAttribute = 'data-new-filename';
  for (const file of document.querySelectorAll('.file-content')) {
    if (file.hasAttribute(filenameAttribute)) {
      const filename = file.getAttribute(filenameAttribute);
      setFileFolding(file, file.querySelector('.fold-file'), filename in viewedFiles || viewedFiles.includes(filename)); // because of this OR, arrays as well as objects are supported
    }
  }
}

