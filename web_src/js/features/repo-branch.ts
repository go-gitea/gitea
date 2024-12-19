import {toggleElem} from '../utils/dom.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';

export function initRepoBranchButton() {
  initRepoCreateBranchButton();
  initRepoRenameBranchButton();
}

function initRepoCreateBranchButton() {
  // 2 pages share this code, one is the branch list page, the other is the commit view page: create branch/tag from current commit (dirty code)
  for (const el of document.querySelectorAll('.show-create-branch-modal')) {
    el.addEventListener('click', () => {
      const modalFormName = el.getAttribute('data-modal-form') || '#create-branch-form';
      const modalForm = document.querySelector<HTMLFormElement>(modalFormName);
      if (!modalForm) return;
      modalForm.action = `${modalForm.getAttribute('data-base-action')}${el.getAttribute('data-branch-from-urlcomponent')}`;

      const fromSpanName = el.getAttribute('data-modal-from-span') || '#modal-create-branch-from-span';
      document.querySelector(fromSpanName).textContent = el.getAttribute('data-branch-from');

      fomanticQuery(el.getAttribute('data-modal')).modal('show');
    });
  }
}

function initRepoRenameBranchButton() {
  for (const el of document.querySelectorAll('.show-rename-branch-modal')) {
    el.addEventListener('click', () => {
      const target = el.getAttribute('data-modal');
      const modal = document.querySelector(target);
      const oldBranchName = el.getAttribute('data-old-branch-name');
      modal.querySelector<HTMLInputElement>('input[name=from]').value = oldBranchName;

      // display the warning that the branch which is chosen is the default branch
      const warn = modal.querySelector('.default-branch-warning');
      toggleElem(warn, el.getAttribute('data-is-default-branch') === 'true');

      const text = modal.querySelector('[data-rename-branch-to]');
      text.textContent = text.getAttribute('data-rename-branch-to').replace('%s', oldBranchName);
    });
  }
}
