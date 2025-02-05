import {initCompLabelEdit} from './comp/LabelEdit.ts';
import {toggleElem} from '../utils/dom.ts';
import {initCompCropper} from './comp/Cropper.ts';

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

  // Avatar Cropper
  if (document.querySelector<HTMLDivElement>('.organization.settings.options')) {
    const fileInput = document.querySelector<HTMLInputElement>('#new-avatar');
    const container = document.querySelector<HTMLElement>('.organization.settings.options .cropper-panel');
    const imageSource = container.querySelector<HTMLImageElement>('.cropper-source');
    initCompCropper({container, fileInput, imageSource});
  }
}
