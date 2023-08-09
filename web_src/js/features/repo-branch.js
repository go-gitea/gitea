import $ from 'jquery';
import {toggleElem} from '../utils/dom.js';

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

    // display the warning that the branch which is chosen is the default branch
    const $warn = $modal.find('.default-branch-warning');
    toggleElem($warn, $(this).attr('data-is-default-branch') === 'true');

    const $text = $modal.find('[data-rename-branch-to]');
    $text.text($text.attr('data-rename-branch-to').replace('%s', oldBranchName));
  });
}
