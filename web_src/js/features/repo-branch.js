import $ from 'jquery';

const { i18n } = window.config;
const renameBranchFromInputSelector = "input#from"
const renameBranchModalSelector = "#rename-branch-modal"
const renameBranchToSpanSelector = "#modal-rename-branch-to-span"

export function initRepoBranchButton() {
  initRepoCreateBranchButton();
  initRepoRenameBranchButton();
}

function initRepoCreateBranchButton() {
  const showCreateBranchModal = $('.show-create-branch-modal');
  if (showCreateBranchModal.length === 0) return;

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
    $($(this).attr('data-modal')).modal('show');
  });
}

function initRepoRenameBranchButton() {
  const showRenameBranchModal = $('.show-rename-branch-modal');
  if (showRenameBranchModal.length === 0) return;

  $('.show-rename-branch-modal').on('click', function () {
    const oldBranchName = $(this).attr('data-old-branch-name');
    $(renameBranchFromInputSelector)?.val(oldBranchName);
    console.log($(renameBranchFromInputSelector));
    $(renameBranchToSpanSelector).text(i18n.rename_branch_to.replace('%s', oldBranchName));
    $(renameBranchModalSelector).modal('show');
  })
}
