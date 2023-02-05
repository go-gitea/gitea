import $ from 'jquery';
import {initCompColorPicker} from './ColorPicker.js';

export function initCompLabelEdit(selector) {
  if (!$(selector).length) return;
  initCompColorPicker();

  // Create label
  $('.new-label.button').on('click', () => {
    $('.new-label.modal').modal({
      onApprove() {
        $('.new-label.form').trigger('submit');
      }
    }).modal('show');
    return false;
  });

  // Edit label
  $('.edit-label-button').on('click', function () {
    $('.edit-label .color-picker').minicolors('value', $(this).data('color'));
    $('#label-modal-id').val($(this).data('id'));
    $('.edit-label .new-label-input').val($(this).data('title'));
    $('.edit-label .new-label-exclusive').prop('checked', $(this).data('exclusive'));
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
