import {createTippy} from '../modules/tippy.ts';
import type {DOMEvent} from '../utils/dom.ts';

export async function initColorPickers() {
  const els = document.querySelectorAll<HTMLElement>('.js-color-picker-input');
  if (!els.length) return;

  await Promise.all([
    import(/* webpackChunkName: "colorpicker" */'vanilla-colorful/hex-color-picker.js'),
    import(/* webpackChunkName: "colorpicker" */'../../css/features/colorpicker.css'),
  ]);

  for (const el of els) {
    initPicker(el);
  }
}

function updateSquare(el: HTMLElement, newValue: string): void {
  el.style.color = /#[0-9a-f]{6}/i.test(newValue) ? newValue : 'transparent';
}

function updatePicker(el: HTMLElement, newValue: string): void {
  el.setAttribute('color', newValue);
}

function initPicker(el: HTMLElement): void {
  const input = el.querySelector('input');

  const square = document.createElement('div');
  square.classList.add('preview-square');
  updateSquare(square, input.value);
  el.append(square);

  const picker = document.createElement('hex-color-picker');
  picker.addEventListener('color-changed', (e) => {
    input.value = e.detail.value;
    input.focus();
    updateSquare(square, e.detail.value);
  });

  input.addEventListener('input', (e: DOMEvent<Event, HTMLInputElement>) => {
    updateSquare(square, e.target.value);
    updatePicker(picker, e.target.value);
  });

  createTippy(input, {
    trigger: 'focus click',
    theme: 'bare',
    hideOnClick: true,
    content: picker,
    placement: 'bottom-start',
    interactive: true,
    onShow() {
      updatePicker(picker, input.value);
    },
  });

  // init precolors
  for (const colorEl of el.querySelectorAll<HTMLElement>('.precolors .color')) {
    colorEl.addEventListener('click', (e: DOMEvent<MouseEvent, HTMLAnchorElement>) => {
      const newValue = e.target.getAttribute('data-color-hex');
      input.value = newValue;
      input.dispatchEvent(new Event('input', {bubbles: true}));
      updateSquare(square, newValue);
    });
  }
}
