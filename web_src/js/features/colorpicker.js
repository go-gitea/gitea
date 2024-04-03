export async function initColorPickers(selector = '.js-color-picker-input input', opts = {}) {
  const inputEls = document.querySelectorAll(selector);
  if (!inputEls.length) return;

  const [{coloris, init}] = await Promise.all([
    import(/* webpackChunkName: "colorpicker" */'@melloware/coloris'),
    import(/* webpackChunkName: "colorpicker" */'../../css/features/colorpicker.css'),
  ]);

  init();
  coloris({
    el: selector,
    alpha: false,
    focusInput: true,
    selectInput: false,
    ...opts,
  });

  for (const inputEl of inputEls) {
    const parent = inputEl.closest('.js-color-picker-input');
    // prevent tabbing on the color preview `button` inside the input
    parent.querySelector('button').tabIndex = -1;
    // init precolors
    for (const el of parent.querySelectorAll('.precolors .color')) {
      el.addEventListener('click', (e) => {
        inputEl.value = e.target.getAttribute('data-color-hex');
        inputEl.dispatchEvent(new Event('input', {bubbles: true}));
      });
    }
  }
}
