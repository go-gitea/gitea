import $ from 'jquery';
import {hideElem, showElem, type DOMEvent} from '../utils/dom.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';

export function initRepoRelease() {
  document.addEventListener('click', (e: DOMEvent<MouseEvent>) => {
    if (e.target.matches('.remove-rel-attach')) {
      const uuid = e.target.getAttribute('data-uuid');
      const id = e.target.getAttribute('data-id');
      document.querySelector<HTMLInputElement>(`input[name='attachment-del-${uuid}']`).value = 'true';
      hideElem(`#attachment-${id}`);
    }
  });
}

export function initRepoReleaseNew() {
  if (!document.querySelector('.repository.new.release')) return;

  initTagNameEditor();
}

function initTagNameEditor() {
  const el = document.querySelector('#tag-name-editor');
  if (!el) return;

  const tagWarning = document.querySelector('#tag-warning');
  const tagWarningDetailLinks = Array.from(document.querySelectorAll('.tag-warning-detail'));
  const existingTags = JSON.parse(el.getAttribute('data-existing-tags'));

  const defaultTagHelperText = el.getAttribute('data-tag-helper');
  const newTagHelperText = el.getAttribute('data-tag-helper-new');
  const existingTagHelperText = el.getAttribute('data-tag-helper-existing');
  const tagURLStub = tagWarning.getAttribute('data-commit-url-stub');
  const tagConfirmDraftModal = document.querySelector('#tag-confirm-draft-modal');
  const tagConfirmModal = document.querySelector('#tag-confirm-modal');

  // show the confirmation modal if release is using an existing tag
  let requiresConfirmation = false;
  $('.tag-confirm').on('click', (event) => {
    if (requiresConfirmation) {
      event.preventDefault();
      const form = event.target.closest('form');
      if (event.target.classList.contains('tag-draft')) {
        fomanticQuery(tagConfirmDraftModal).modal({
          onApprove() {
            // need to add hidden input with draft form value
            // (triggering form submission doesn't include the button data)
            const input = document.createElement('input');
            input.type = 'hidden';
            input.name = 'draft';
            input.value = '1';
            form.append(input);
            $(form).trigger('submit');
          },
        }).modal('show');
      } else {
        fomanticQuery(tagConfirmModal).modal({
          onApprove() {
            $(form).trigger('submit');
          },
        }).modal('show');
      }
    }
  });

  const tagNameInput = document.querySelector<HTMLInputElement>('#tag-name');
  const hideTargetInput = function(tagNameInput: HTMLInputElement) {
    const value = tagNameInput.value;
    const tagHelper = document.querySelector('#tag-helper');
    if (value in existingTags) {
      // If the tag already exists, hide the target branch selector.
      hideElem('#tag-target-selector');
      tagHelper.textContent = existingTagHelperText;
      showElem('#tag-warning');
      for (const detail of tagWarningDetailLinks) {
        const anchor = detail as HTMLAnchorElement;
        anchor.href = `${tagURLStub}/${existingTags[value]}`;
        anchor.textContent = existingTags[value].substring(0, 10);
      }
      requiresConfirmation = true;
    } else {
      showElem('#tag-target-selector');
      tagHelper.textContent = value ? newTagHelperText : defaultTagHelperText;
      hideElem('#tag-warning');
      requiresConfirmation = false;
    }
  };
  hideTargetInput(tagNameInput); // update on page load because the input may have a value
  tagNameInput.addEventListener('input', (e) => {
    hideTargetInput(e.target as HTMLInputElement);
  });
}
