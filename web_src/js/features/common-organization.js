import $ from 'jquery';
import {initCompLabelEdit} from './comp/LabelEdit.js';
import {toggleElem} from '../utils/dom.js';

export function initCommonOrganization() {
  if ($('.organization').length === 0) {
    return;
  }

  $('.organization.settings.options #org_name').on('input', function () {
    const nameChanged = $(this).val().toLowerCase() !== $(this).attr('data-org-name').toLowerCase();
    toggleElem('#org-name-change-prompt', nameChanged);
  });

  // Labels
  initCompLabelEdit('.organization.settings.labels');
}
