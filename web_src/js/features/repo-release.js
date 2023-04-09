import $ from 'jquery';
import {hideElem, showElem} from '../utils/dom.js';
import {initComboMarkdownEditor} from './comp/ComboMarkdownEditor.js';

export function initRepoRelease() {
  $(document).on('click', '.remove-rel-attach', function() {
    const uuid = $(this).data('uuid');
    const id = $(this).data('id');
    $(`input[name='attachment-del-${uuid}']`).attr('value', true);
    hideElem($(`#attachment-${id}`));
  });
}

export function initRepoReleaseNew() {
  const $repoReleaseNew = $('.repository.new.release');
  if (!$repoReleaseNew.length) return;

  initTagNameEditor();
  initRepoReleaseEditor();
}

function initTagNameEditor() {
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

function initRepoReleaseEditor() {
  const $editor = $('.repository.new.release .combo-markdown-editor');
  if ($editor.length === 0) {
    return;
  }
  const _promise = initComboMarkdownEditor($editor);
}
