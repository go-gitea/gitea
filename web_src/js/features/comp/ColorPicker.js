import $ from 'jquery';
import createColorPicker from '../colorpicker.js';

export function initCompColorPicker() {
  createColorPicker($('.color-picker'));

  $('.precolors .color').on('click', function () {
    const color_hex = $(this).data('color-hex');
    $('.color-picker').val(color_hex);
    $('.minicolors-swatch-color').css('background-color', color_hex);
  });
}
