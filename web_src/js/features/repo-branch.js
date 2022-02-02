import $ from 'jquery';

export function initRepoBranchButton() {
  $('.show-create-branch-modal').on('click', function () {
    let modalFormName = $(this).data('modal-form');
    if (!modalFormName) {
      modalFormName = '#create-branch-form';
    }
    $(modalFormName)[0].action = $(modalFormName).data('base-action') + $(this).data('branch-from-urlcomponent');
    let fromSpanName = $(this).data('modal-from-span');
    if (!fromSpanName) {
      fromSpanName = '#modal-create-branch-from-span';
    }

    $(fromSpanName).text($(this).data('branch-from'));
    $($(this).data('modal')).modal('show');
  });
}
