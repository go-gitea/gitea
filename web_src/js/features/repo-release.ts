import {POST} from '../modules/fetch.ts';
import {hideToastsAll, showErrorToast} from '../modules/toast.ts';
import {getComboMarkdownEditor} from './comp/ComboMarkdownEditor.ts';
import {hideElem, showElem} from '../utils/dom.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {registerGlobalEventFunc, registerGlobalInitFunc} from '../modules/observer.ts';

export function initRepoReleaseNew() {
  registerGlobalEventFunc('click', 'onReleaseEditAttachmentDelete', (el) => {
    const uuid = el.getAttribute('data-uuid')!;
    const id = el.getAttribute('data-id')!;
    document.querySelector<HTMLInputElement>(`input[name='attachment-del-${uuid}']`)!.value = 'true';
    hideElem(`#attachment-${id}`);
  });
  registerGlobalInitFunc('initReleaseEditForm', (elForm: HTMLFormElement) => {
    initTagNameEditor(elForm);
    initGenerateReleaseNotes(elForm);
  });
}

function initTagNameEditor(elForm: HTMLFormElement) {
  const tagNameInput = elForm.querySelector<HTMLInputElement>('input[type=text][name=tag_name]');
  if (!tagNameInput) return; // only init if tag name input exists (the tag name is editable)

  const existingTags = JSON.parse(elForm.getAttribute('data-existing-tags')!);
  const defaultTagHelperText = elForm.getAttribute('data-tag-helper');
  const newTagHelperText = elForm.getAttribute('data-tag-helper-new');
  const existingTagHelperText = elForm.getAttribute('data-tag-helper-existing');

  const hideTargetInput = function(tagNameInput: HTMLInputElement) {
    const value = tagNameInput.value;
    const tagHelper = elForm.querySelector('.tag-name-helper')!;
    if (existingTags.includes(value)) {
      // If the tag already exists, hide the target branch selector.
      hideElem(elForm.querySelectorAll('.tag-target-selector'));
      tagHelper.textContent = existingTagHelperText;
    } else {
      showElem(elForm.querySelectorAll('.tag-target-selector'));
      tagHelper.textContent = value ? newTagHelperText : defaultTagHelperText;
    }
  };
  hideTargetInput(tagNameInput); // update on page load because the input may have a value
  tagNameInput.addEventListener('input', (e) => {
    hideTargetInput(e.target as HTMLInputElement);
  });
}

function initGenerateReleaseNotes(elForm: HTMLFormElement) {
  const buttonShowModal = elForm.querySelector<HTMLButtonElement>('.button.generate-release-notes')!;
  const tagNameInput = elForm.querySelector<HTMLInputElement>('input[name=tag_name]')!;
  const targetInput = elForm.querySelector<HTMLInputElement>('input[name=tag_target]')!;

  const textMissingTag = buttonShowModal.getAttribute('data-text-missing-tag')!;
  const generateUrl = buttonShowModal.getAttribute('data-generate-url')!;

  const elModal = document.querySelector('#generate-release-notes-modal')!;

  const doSubmit = async (tagName: string) => {
    const elPreviousTag = elModal.querySelector<HTMLSelectElement>('[name=previous_tag]')!;
    const comboEditor = getComboMarkdownEditor(elForm.querySelector<HTMLElement>('.combo-markdown-editor'))!;

    const form = new URLSearchParams();
    form.set('tag_name', tagName);
    form.set('tag_target', targetInput.value);
    form.set('previous_tag', elPreviousTag.value);

    elModal.classList.add('loading', 'disabled');
    try {
      const resp = await POST(generateUrl, {data: form});
      const data = await resp.json();
      if (!resp.ok) {
        showErrorToast(data.errorMessage || resp.statusText);
        return;
      }
      const oldValue = comboEditor.value().trim();
      if (oldValue) {
        // Don't overwrite existing content. Maybe in the future we can let users decide: overwrite or append or copy-to-clipboard
        // GitHub just disables the button if the content is not empty
        comboEditor.value(`${oldValue}\n\n${data.content}`);
      } else {
        comboEditor.value(data.content);
      }
    } finally {
      elModal.classList.remove('loading', 'disabled');
      fomanticQuery(elModal).modal('hide');
      comboEditor.focus();
    }
  };

  const doShowModal = () => {
    hideToastsAll();
    const tagName = tagNameInput.value.trim();
    if (!tagName) {
      showErrorToast(textMissingTag, {duration: 3000});
      tagNameInput.focus();
      return;
    }
    fomanticQuery(elModal).modal({
      onApprove: () => {
        doSubmit(tagName); // don't await, need to return false to keep the modal
        return false;
      },
    }).modal('show');
  };

  buttonShowModal.addEventListener('click', doShowModal);
}
