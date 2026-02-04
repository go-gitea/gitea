import {createTippy} from '../modules/tippy.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';

export async function initColorPickers() {
  let imported = false;
  registerGlobalInitFunc('initColorPicker', async (el) => {
    if (!imported) {
      await Promise.all([
        import(/* webpackChunkName: "colorpicker" */'vanilla-colorful/hex-color-picker.js'),
        import(/* webpackChunkName: "colorpicker" */'../../css/features/colorpicker.css'),
      ]);
      imported = true;
    }
    initPicker(el);
  });
}

function updateSquare(el: HTMLElement, newValue: string): void {
  el.style.color = /#[0-9a-f]{6}/i.test(newValue) ? newValue : 'transparent';
}

function updatePicker(el: HTMLElement, newValue: string): void {
  el.setAttribute('color', newValue);
}

function initPicker(el: HTMLElement): void {
  const input = el.querySelector('input')!;

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

  input.addEventListener('input', (e) => {
    updateSquare(square, (e.target as HTMLInputElement).value);
    updatePicker(picker, (e.target as HTMLInputElement).value);
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

  // init random color & precolors
  const setSelectedColor = (color: string) => {
    input.value = color;
    input.dispatchEvent(new Event('input', {bubbles: true}));
    updateSquare(square, color);
  };
  el.querySelector('.generate-random-color')!.addEventListener('click', () => {
    const newValue = `#${Math.floor(Math.random() * 0xFFFFFF).toString(16).padStart(6, '0')}`;
    setSelectedColor(newValue);
  });
  for (const colorEl of el.querySelectorAll<HTMLElement>('.precolors .color')) {
    colorEl.addEventListener('click', (e) => {
      const newValue = (e.target as HTMLElement).getAttribute('data-color-hex')!;
      setSelectedColor(newValue);
    });
  }
}
