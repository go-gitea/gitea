import {POST} from '../modules/fetch.ts';
import {hideToastsAll, showErrorToast} from '../modules/toast.ts';
import {getComboMarkdownEditor} from './comp/ComboMarkdownEditor.ts';
import {hideElem, showElem} from '../utils/dom.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {registerGlobalEventFunc, registerGlobalInitFunc} from '../modules/observer.ts';
import {htmlEscape} from '../utils/html.ts';
import {compareVersions} from 'compare-versions';

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

function getReleaseFormExistingTags(elForm: HTMLFormElement): Array<string> {
  return JSON.parse(elForm.getAttribute('data-existing-tags')!);
}

function initTagNameEditor(elForm: HTMLFormElement) {
  const tagNameInput = elForm.querySelector<HTMLInputElement>('input[type=text][name=tag_name]');
  if (!tagNameInput) return; // only init if tag name input exists (the tag name is editable)

  const existingTags = getReleaseFormExistingTags(elForm);
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

export function guessPreviousReleaseTag(tagName: string, existingTags: Array<string>): string {
  let guessedPreviousTag = '', guessedPreviousVer = '';

  const cleanup = (s: string) => {
    const pos = s.lastIndexOf('/');
    if (pos >= 0) s = s.substring(pos + 1);
    if (s.substring(0, 1).toLowerCase() === 'v') s = s.substring(1);
    return s;
  };

  const newVer = cleanup(tagName);
  for (const s of existingTags) {
    const existingVer = cleanup(s);
    try {
      if (compareVersions(existingVer, newVer) >= 0) continue;
      if (!guessedPreviousTag || compareVersions(existingVer, guessedPreviousVer) > 0) {
        guessedPreviousTag = s;
        guessedPreviousVer = existingVer;
      }
    } catch {}
  }
  return guessedPreviousTag;
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

  let inited = false;
  const doShowModal = () => {
    hideToastsAll();
    const tagName = tagNameInput.value.trim();
    if (!tagName) {
      showErrorToast(textMissingTag, {duration: 3000});
      tagNameInput.focus();
      return;
    }

    const existingTags = getReleaseFormExistingTags(elForm);
    const $dropdown = fomanticQuery(elModal.querySelector('[name=previous_tag]')!);
    if (!inited) {
      inited = true;
      const values = [];
      for (const tagName of existingTags) {
        values.push({name: htmlEscape(tagName), value: tagName}); // ATTENTION: dropdown takes the "name" input as raw HTML
      }
      $dropdown.dropdown('change values', values);
    }
    $dropdown.dropdown('set selected', guessPreviousReleaseTag(tagName, existingTags));

    fomanticQuery(elModal).modal({
      onApprove: () => {
        doSubmit(tagName); // don't await, need to return false to keep the modal
        return false;
      },
    }).modal('show');
  };

  buttonShowModal.addEventListener('click', doShowModal);
}
