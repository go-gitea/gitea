import tippy from 'tippy.js';

export function createTippy(target, opts = {}) {
  const instance = tippy(target, {
    appendTo: document.body,
    animation: false,
    allowHTML: false,
    hideOnClick: false,
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

function getTippyTooltipContent(target) {
  // prefer to always use the "[data-tooltip-content]" attribute
  // for backward compatibility, we also support the ".tooltip[data-content]" attribute
  let content = target.getAttribute('data-tooltip-content');
  if (!content && target.classList.contains('tooltip')) {
    content = target.getAttribute('data-content');
  }
  return content;
}

/**
 * Attach a tippy tooltip to the given target element.
 * If the target element already has a tippy tooltip attached, the tooltip will be updated with the new content.
 * If the target element has no content, then no tooltip will be attached, and it returns null.
 * @param target {HTMLElement}
 * @param content {null|string}
 * @returns {null|tippy}
 */
function attachTippyTooltip(target, content = null) {
  content = content ?? getTippyTooltipContent(target);
  if (!content) return null;

  const props = {
    content,
    delay: 100,
    role: 'tooltip',
    placement: target.getAttribute('data-tooltip-placement') || 'top-start',
    ...(target.getAttribute('data-tooltip-interactive') === 'true' ? {interactive: true} : {}),
  };

  if (!target._tippy) {
    createTippy(target, props);
  } else {
    target._tippy.setProps(props);
  }
  return target._tippy;
}

/**
 * creating tippy instance is expensive, so we only create it when the user hovers over the element
 * @param e {Event}
 */
function lazyTippyOnMouseEnter(e) {
  e.target.removeEventListener('mouseenter', lazyTippyOnMouseEnter, true);
  attachTippyTooltip(this);
}

/**
 * Activate the tippy tooltip for all children elements
 * And if the element has no aria-label, use the tooltip content as aria-label
 * @param target {HTMLElement}
 */
function attachChildrenLazyTippyTooltip(target) {
  // the selector must match the logic in getTippyTooltipContent
  for (const el of target.querySelectorAll('[data-tooltip-content], .tooltip[data-content]')) {
    el.addEventListener('mouseenter', lazyTippyOnMouseEnter, true);

    // meanwhile, if the element has no aria-label, use the tooltip content as aria-label
    if (!el.hasAttribute('aria-label')) {
      const content = getTippyTooltipContent(el);
      if (content) {
        el.setAttribute('aria-label', content);
      }
    }
  }
}

export function initGlobalTooltips() {
  // use MutationObserver to detect new elements added to the DOM, or attributes changed
  const observer = new MutationObserver((mutationList) => {
    for (const mutation of mutationList) {
      if (mutation.type === 'childList') {
        for (const el of mutation.addedNodes) {
          // handle all "tooltip" elements in newly added nodes, skip non-related nodes (eg: "#text")
          if (el.querySelectorAll) {
            attachChildrenLazyTippyTooltip(el);
          }
        }
      } else if (mutation.type === 'attributes') {
        // sync the tooltip content if the attributes change
        attachTippyTooltip(mutation.target);
      }
    }
  });
  observer.observe(document, {
    subtree: true,
    childList: true,
    attributeFilter: ['data-tooltip-content', 'data-content'],
  });

  attachChildrenLazyTippyTooltip(document.documentElement);
}

export function showTemporaryTooltip(target, content) {
  const tippy = target._tippy ?? attachTippyTooltip(target, content);
  tippy.setContent(content);
  if (!tippy.state.isShown) tippy.show();
  tippy.setProps({
    onHidden: (tippy) => {
      // reset the default tooltip content, if no default, then this temporary tooltip could be destroyed
      if (!attachTippyTooltip(target)) {
        tippy.destroy();
      }
    },
  });
}
