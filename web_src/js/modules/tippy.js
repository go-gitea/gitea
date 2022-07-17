import tippy from 'tippy.js';

export function createTippy(target, opts = {}) {
  return tippy(target, {
    appendTo: document.body,
    placement: 'top-start',
    animation: false,
    allowHTML: true,
    arrow: `<svg width="16" height="6"><path d="m0 6 8-6 8 6Z" class="tippy-svg-arrow-outer"/><path d="m0 7 8-6 8 6Z" class="tippy-svg-arrow-inner"/></svg>`,
    ...opts,
  });
}
