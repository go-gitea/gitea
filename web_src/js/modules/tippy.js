import tippy from 'tippy.js';

export function createTippy(target, opts = {}) {
  const instance = tippy(target, {
    appendTo: document.body,
    placement: target.getAttribute('data-placement') || 'top-start',
    animation: false,
    allowHTML: false,
    interactiveBorder: 30,
    ignoreAttributes: true,
    maxWidth: 500, // increase over default 350px
    arrow: `<svg width="16" height="7"><path d="m0 7 8-7 8 7Z" class="tippy-svg-arrow-outer"/><path d="m0 8 8-7 8 7Z" class="tippy-svg-arrow-inner"/></svg>`,
    ...(opts?.role && {theme: opts.role}),
    ...opts,
  });

  // for popups where content refers to a DOM element, we use the 'tippy-target' class
  // to initially hide the content, now we can remove it as the content has been removed
  // from the DOM by tippy
  if (opts.content instanceof Element) {
    opts.content.classList.remove('tippy-target');
  }

  return instance;
}

export function initTooltip(el, props = {}) {
  const content = el.getAttribute('data-content') || props.content;
  if (!content) return null;
  return createTippy(el, {
    content,
    delay: 100,
    role: 'tooltip',
    ...props,
  });
}

export function showTemporaryTooltip(target, content) {
  let tippy, oldContent;
  if (target._tippy) {
    tippy = target._tippy;
    oldContent = tippy.props.content;
  } else {
    tippy = initTooltip(target, {content});
  }

  tippy.setContent(content);
  tippy.show();
  tippy.setProps({
    onHidden: (tippy) => {
      if (oldContent) {
        tippy.setContent(oldContent);
      } else {
        tippy.destroy();
      }
      tippy.setProps({onHidden: undefined});
    },
  });
}
