import {initCompColorPicker} from './ColorPicker.js';

export function initCompLabelEdit(selector) {
  if (!$.find(selector).length) return;
  // Create label
  const $newLabelPanel = $('.new-label.segment');
  $('.new-label.button').on('click', () => {
    $newLabelPanel.show();
  });
  $('.new-label.segment .cancel').on('click', () => {
    $newLabelPanel.hide();
  });

  initCompColorPicker();

  $('.edit-label-button').on('click', function () {
    $('.edit-label .color-picker').minicolors('value', $(this).data('color'));
    $('#label-modal-id').val($(this).data('id'));
    $('.edit-label .new-label-input').val($(this).data('title'));
    $('.edit-label .new-label-desc-input').val($(this).data('description'));
    $('.edit-label .color-picker').val($(this).data('color'));
    $('.edit-label .minicolors-swatch-color').css('background-color', $(this).data('color'));
    $('.edit-label.modal').modal({
      onApprove() {
        $('.edit-label.form').trigger('submit');
      }
    }).modal('show');
    return false;
  });
}
