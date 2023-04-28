import $ from 'jquery';

const {i18n_branch} = window.config;
const renameBranchFromInputSelector = 'input#from';
const renameBranchToSpanSelector = '#modal-rename-branch-to-span';

export function initRepoBranchButton() {
  initRepoCreateBranchButton();
  initRepoRenameBranchButton();
}

function initRepoCreateBranchButton() {
  $('.show-create-branch-modal').on('click', function () {
    let modalFormName = $(this).attr('data-modal-form');
    if (!modalFormName) {
      modalFormName = '#create-branch-form';
    }
    $(modalFormName)[0].action = $(modalFormName).attr('data-base-action') + $(this).attr('data-branch-from-urlcomponent');
    let fromSpanName = $(this).attr('data-modal-from-span');
    if (!fromSpanName) {
      fromSpanName = '#modal-create-branch-from-span';
    }

    $(fromSpanName).text($(this).attr('data-branch-from'));
  });
}

function initRepoRenameBranchButton() {
  $('.show-rename-branch-modal').on('click', function () {
    const oldBranchName = $(this).attr('data-old-branch-name');
    $(renameBranchFromInputSelector)?.val(oldBranchName);
    $(renameBranchToSpanSelector).text(i18n_branch.rename_branch_to.replace('%s', oldBranchName));
  });
}
