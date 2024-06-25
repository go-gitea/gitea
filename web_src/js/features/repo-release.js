import {hideElem, showElem} from '../utils/dom.js';
import {initComboMarkdownEditor} from './comp/ComboMarkdownEditor.js';

export function initRepoRelease() {
  document.addEventListener('click', (e) => {
    if (e.target.matches('.remove-rel-attach')) {
      const uuid = e.target.getAttribute('data-uuid');
      const id = e.target.getAttribute('data-id');
      document.querySelector(`input[name='attachment-del-${uuid}']`).value = 'true';
      hideElem(`#attachment-${id}`);
    }
  });
}

export function initRepoReleaseNew() {
  if (!document.querySelector('.repository.new.release')) return;

  initTagNameEditor();
  initRepoReleaseEditor();
}

function initTagNameEditor() {
  const el = document.querySelector('#tag-name-editor');
  if (!el) return;

  const existingTags = JSON.parse(el.getAttribute('data-existing-tags'));
  if (!Array.isArray(existingTags)) return;

  const defaultTagHelperText = el.getAttribute('data-tag-helper');
  const newTagHelperText = el.getAttribute('data-tag-helper-new');
  const existingTagHelperText = el.getAttribute('data-tag-helper-existing');

  const tagNameInput = document.querySelector('#tag-name');
  const hideTargetInput = function(tagNameInput) {
    const value = tagNameInput.value;
    const tagHelper = document.querySelector('#tag-helper');
    if (existingTags.includes(value)) {
      // If the tag already exists, hide the target branch selector.
      hideElem('#tag-target-selector');
      tagHelper.textContent = existingTagHelperText;
    } else {
      showElem('#tag-target-selector');
      tagHelper.textContent = value ? newTagHelperText : defaultTagHelperText;
    }
  };
  hideTargetInput(tagNameInput); // update on page load because the input may have a value
  tagNameInput.addEventListener('input', (e) => {
    hideTargetInput(e.target);
  });
}

function initRepoReleaseEditor() {
  const editor = document.querySelector('.repository.new.release .combo-markdown-editor');
  if (!editor) {
    return;
  }
  initComboMarkdownEditor(editor);
}
