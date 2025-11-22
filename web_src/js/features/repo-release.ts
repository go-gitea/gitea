import {POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {getComboMarkdownEditor} from './comp/ComboMarkdownEditor.ts';
import {hideElem, showElem, type DOMEvent} from '../utils/dom.ts';

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
  initGenerateReleaseNotes();
}

function initTagNameEditor() {
  const el = document.querySelector('#tag-name-editor');
  if (!el) return;

  const existingTags = JSON.parse(el.getAttribute('data-existing-tags'));
  if (!Array.isArray(existingTags)) return;

  const defaultTagHelperText = el.getAttribute('data-tag-helper');
  const newTagHelperText = el.getAttribute('data-tag-helper-new');
  const existingTagHelperText = el.getAttribute('data-tag-helper-existing');

  const tagNameInput = document.querySelector<HTMLInputElement>('#tag-name');
  const hideTargetInput = function(tagNameInput: HTMLInputElement) {
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
    hideTargetInput(e.target as HTMLInputElement);
  });
}

function initGenerateReleaseNotes() {
  const button = document.querySelector<HTMLButtonElement>('#generate-release-notes');
  if (!button) return;

  const tagNameInput = document.querySelector<HTMLInputElement>('#tag-name');
  const targetInput = document.querySelector<HTMLInputElement>("input[name='tag_target']");
  const previousTagSelect = document.querySelector<HTMLSelectElement>('#release-previous-tag');
  const missingTagMessage = button.getAttribute('data-missing-tag-message') || 'Tag name is required';
  const generateUrl = button.getAttribute('data-generate-url');

  button.addEventListener('click', async () => {
    const tagName = tagNameInput.value.trim();

    if (!tagName) {
      showErrorToast(missingTagMessage);
      tagNameInput?.focus();
      return;
    }

    const form = new URLSearchParams();
    form.set('tag_name', tagName);
    form.set('tag_target', targetInput.value || '');
    form.set('previous_tag', previousTagSelect.value || '');

    button.classList.add('loading', 'disabled');
    try {
      const resp = await POST(generateUrl, {
        data: form,
      });

      const data = await resp.json();

      if (!resp.ok) {
        throw new Error(data.errorMessage || resp.statusText);
      }
      previousTagSelect.value = data.previous_tag;
      previousTagSelect.dispatchEvent(new Event('change', {bubbles: true}));

      applyGeneratedReleaseNotes(data.content);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      showErrorToast(message);
    } finally {
      button.classList.remove('loading', 'disabled');
    }
  });
}

function applyGeneratedReleaseNotes(content: string) {
  const editorContainer = document.querySelector<HTMLElement>('.combo-markdown-editor');

  const comboEditor = getComboMarkdownEditor(editorContainer);
  comboEditor.value(content);
}
