import {initCompLabelEdit} from './comp/LabelEdit.ts';
import {queryElems, toggleElem} from '../utils/dom.ts';
import {initAvatarUploaderWithCropper} from './comp/Cropper.ts';

export function initCommonOrganization() {
  if (!document.querySelectorAll('.organization').length) {
    return;
  }

  document.querySelector<HTMLInputElement>('.organization.settings.options #org_name')?.addEventListener('input', function () {
    const nameChanged = this.value.toLowerCase() !== this.getAttribute('data-org-name').toLowerCase();
    toggleElem('#org-name-change-prompt', nameChanged);
  });

  // Labels
  initCompLabelEdit('.page-content.organization.settings.labels');

  queryElems(document, '.avatar-file-with-cropper', initAvatarUploaderWithCropper);
}
