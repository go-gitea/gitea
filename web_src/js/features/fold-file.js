import {svg} from '../svg.js';

/*
 * Hides the file if currently present, and shows it otherwise. The actual hiding is performed using CSS.
 *
 * The fold arrow is the icon displayed on the upper left of the file box, especially intended for components having the 'fold-file' class.
 * The file content box is the box that should be hidden or shown, especially intended for components having the 'file-content' class.
 */
export function invertFileFolding(fileContentBox, foldArrow) {
    const newFold = fileContentBox.getAttribute('data-folded') !== 'true';
    foldArrow.innerHTML = svg(`octicon-chevron-${newFold ? 'right' : 'down'}`, 18);
    fileContentBox.setAttribute('data-folded', String(newFold));
}
