import {hideElem, showElem} from '../utils/dom.js';

export function initTagNameEditor() {
  const el = document.getElementById('tag-name-editor');
  if (!el) return;

  const existingTags = JSON.parse(el.getAttribute('data-existing-tags'));
  if (!Array.isArray(existingTags)) return;

  const defaultTagHelperText = el.getAttribute('data-tag-helper');
  const newTagHelperText = el.getAttribute('data-tag-helper-new');
  const existingTagHelperText = el.getAttribute('data-tag-helper-existing');

  document.getElementById('tag-name').addEventListener('keyup', (e) => {
    const value = e.target.value;
    if (existingTags.includes(value)) {
      // If the tag already exists, hide the target branch selector.
      hideElem('#tag-target-selector');
      document.getElementById('tag-helper').innerText = existingTagHelperText;
    } else {
      showElem('#tag-target-selector');
      if (value) {
        document.getElementById('tag-helper').innerText = newTagHelperText;
      } else {
        document.getElementById('tag-helper').innerText = defaultTagHelperText;
      }
    }
  });
}
