import {initCompLabelEdit} from './comp/LabelEdit.js';
import {toggleElem} from '../utils/dom.js';

export function initCommonOrganization() {
  if (!document.querySelectorAll('.organization').length) {
    return;
  }

  document.querySelector('.organization.settings.options #org_name')?.addEventListener('input', function () {
    const nameChanged = this.value.toLowerCase() !== this.getAttribute('data-org-name').toLowerCase();
    toggleElem('#org-name-change-prompt', nameChanged);
  });

  // Labels
  initCompLabelEdit('.organization.settings.labels');
}
