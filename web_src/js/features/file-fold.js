import {svg} from '../svg.js';

// Hides the file if newFold is true, and shows it otherwise. The actual hiding is performed using CSS.
//
// The fold arrow is the icon displayed on the upper left of the file box, especially intended for components having the 'fold-file' class.
// The file content box is the box that should be hidden or shown, especially intended for components having the 'file-content' class.
//
export function setFileFolding(fileContentBox, foldArrow, newFold, isFromViewed = false) {
  const diffFileHeader = fileContentBox.querySelector('.diff-file-header');
  const isFolded = fileContentBox.getAttribute('data-folded');
  console.log('newFold', newFold)
  foldArrow.innerHTML = svg(`octicon-chevron-${newFold ? 'right' : 'down'}`, 18);
  fileContentBox.setAttribute('data-folded', newFold);
  // scroll position needs to be adjusted only when folding the file
  // and scrollY is greater than current file header's offsetTop
  if (isFolded === 'false' && window.scrollY > diffFileHeader.offsetTop) {
    // if the file is folded by clicking the "fold file" icon, scroll to current file header
    let scrollTargetoffsetTop = fileContentBox.offsetTop;
    if (isFromViewed) {
      // if the file is folded by clicking viewed, scroll to next file header
      const nextDiffBox = fileContentBox.nextElementSibling;
      scrollTargetoffsetTop = nextDiffBox.offsetTop;
    }
    window.scrollTo({
      top: scrollTargetoffsetTop - document.querySelector('.diff-detail-box').offsetHeight,
      behavior: 'instant'
    });
  }
}

// Like `setFileFolding`, except that it automatically inverts the current file folding state.
export function invertFileFolding(fileContentBox, foldArrow) {  
  setFileFolding(fileContentBox, foldArrow, fileContentBox.getAttribute('data-folded') !== 'true');
}
