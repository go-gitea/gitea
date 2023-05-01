import $ from 'jquery';

export function initRepoBranchButton() {
  initRepoCreateBranchButton();
  initRepoRenameBranchButton();
}

function initRepoCreateBranchButton() {
  // 2 pages share this code, one is the branch list page, the other is the commit view page: create branch/tag from current commit (dirty code)
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
  $('.show-rename-branch-modal').on('click', function () {
    const target = $(this).attr('data-modal');
    const $modal = $(target);

    const oldBranchName = $(this).attr('data-old-branch-name');
    $modal.find('input[name=from]').val(oldBranchName);

    const $text = $modal.find('[data-rename-branch-to]');
    $text.text($text.attr('data-rename-branch-to').replace('%s', oldBranchName));
  });
}
