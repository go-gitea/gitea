import $ from 'jquery';
import {toggleElem} from '../utils/dom.js';

export function initRepoBranchButton() {
  initRepoCreateBranchButton();
  initRepoRenameBranchButton();
}

function initRepoCreateBranchButton() {
  // 2 pages share this code, one is the branch list page, the other is the commit view page: create branch/tag from current commit (dirty code)
  for (const element of document.querySelectorAll('.show-create-branch-modal')) {
    element.addEventListener('click', () => {
      let modalFormName = element.getAttribute('data-modal-form');
      if (!modalFormName) {
        modalFormName = '#create-branch-form';
      }
      const modalForm = document.querySelector(modalFormName);
      if (!modalForm) return;
      modalForm.action = `${modalForm.getAttribute('data-base-action')}${element.getAttribute('data-branch-from-urlcomponent')}`;

      let fromSpanName = element.getAttribute('data-modal-from-span');
      if (!fromSpanName) {
        fromSpanName = '#modal-create-branch-from-span';
      }
      document.querySelector(fromSpanName).textContent = element.getAttribute('data-branch-from');

      $(element.getAttribute('data-modal')).modal('show');
    });
  }
}

function initRepoRenameBranchButton() {
  for (const element of document.querySelectorAll('.show-rename-branch-modal')) {
    element.addEventListener('click', () => {
      const target = element.getAttribute('data-modal');
      const modal = document.querySelector(target);
      const oldBranchName = element.getAttribute('data-old-branch-name');
      modal.querySelector('input[name=from]').value = oldBranchName;

      // display the warning that the branch which is chosen is the default branch
      const warn = modal.querySelector('.default-branch-warning');
      toggleElem(warn, element.getAttribute('data-is-default-branch') === 'true');

      const text = modal.querySelector('[data-rename-branch-to]');
      text.textContent = text.getAttribute('data-rename-branch-to').replace('%s', oldBranchName);
    });
  }
}
