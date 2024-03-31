import {createTippy} from '../modules/tippy.js';

export async function initColorPickers() {
  const els = document.querySelectorAll('.js-color-picker-input');
  if (!els.length) return;

  await Promise.all([
    import(/* webpackChunkName: "colorpicker" */'vanilla-colorful/hex-color-picker.js'),
    import(/* webpackChunkName: "colorpicker" */'../../css/features/colorpicker.css'),
  ]);

  for (const el of els) {
    initPicker(el);
  }
}

function updateSquare(el, newValue) {
  el.style.color = /#[0-9a-f]{6}/i.test(newValue) ? newValue : 'transparent';
}

function initPicker(el) {
  const input = el.querySelector('input');
  const previewSquare = document.createElement('div');
  previewSquare.classList.add('preview-square');
  updateSquare(previewSquare, input.value);
  el.append(previewSquare);

  const picker = document.createElement('hex-color-picker');
  picker.setAttribute('data-color', input.value);

  picker.addEventListener('color-changed', (e) => {
    input.value = e.detail.value;
    input.focus();
    updateSquare(previewSquare, e.detail.value);
  });

  input.addEventListener('input', (e) => {
    updateSquare(previewSquare, e.target.value);
  });

  createTippy(input, {
    trigger: 'focus click',
    theme: 'bare',
    hideOnClick: true,
    content: picker,
    placement: 'bottom-start',
    interactive: true,
  });

  // init precolors
  for (const colorEl of el.querySelectorAll('.precolors .color')) {
    colorEl.addEventListener('click', (e) => {
      input.value = e.target.getAttribute('data-color-hex');
      input.dispatchEvent(new Event('input', {bubbles: true}));
      previewSquare.style.color = input.value;
    });
  }
}
