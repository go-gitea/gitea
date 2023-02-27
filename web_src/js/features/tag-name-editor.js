import $ from 'jquery';
import {hideElem, showElem} from '../utils/dom.js';

export function initTagNameEditor() {
  const el = document.getElementById('tag-name-editor');
  if (!el) return;

  const existingTags = JSON.parse(el.getAttribute('data-existing-tags'));
  if (!Array.isArray(existingTags)) return;

  const defaultTagHelperText = el.getAttribute('data-tag-helper');
  const newTagHelperText = el.getAttribute('data-tag-helper-new');
  const existingTagHelperText = el.getAttribute('data-tag-helper-existing');

  $('#tag-name').on('keyup', (e) => {
    const value = e.target.value;
    if (existingTags.includes(value)) {
      // If the tag already exists, hide the target branch selector.
      hideElem($('#tag-target-selector'));
      $('#tag-helper').text(existingTagHelperText);
    } else {
      showElem($('#tag-target-selector'));
      if (typeof value === 'string' && value.length > 0) {
        $('#tag-helper').text(newTagHelperText);
      } else {
        $('#tag-helper').text(defaultTagHelperText);
      }
    }
  });
}
