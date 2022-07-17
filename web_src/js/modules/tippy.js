import tippy from 'tippy.js';

export function createTippy(target, opts) {
  return tippy(target, {
    appendTo: document.body,
    placement: 'top-start',
    animation: false,
    allowHTML: true,
    arrow: `<svg width="16" height="7"><path d="m0 7 8-7 8 7Z" class="tippy-svg-arrow-outer"/><path d="m0 8 8-7 8 7Z" class="tippy-svg-arrow-inner"/></svg>`,
    ...opts,
  });
}
