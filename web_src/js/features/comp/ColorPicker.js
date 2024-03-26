import $ from 'jquery';
import {createColorPicker} from '../colorpicker.js';

export function initCompColorPicker() {
  (async () => {
    await createColorPicker(document.querySelectorAll('.color-picker'));

    for (const el of document.querySelectorAll('.precolors .color')) {
      el.addEventListener('click', (e) => {
        const color = e.target.getAttribute('data-color-hex');
        const parent = e.target.closest('.color.picker');
        $(parent.querySelector('.color-picker')).minicolors('value', color);
      });
    }
  })();
}
