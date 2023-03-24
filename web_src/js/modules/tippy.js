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

/**
 * Attach a tooltip tippy to the given target element.
 * If the target element already has a tooltip tippy attached, the tooltip will be updated with the new content.
 * If the target element has no content, then no tooltip will be attached, and it returns null.
 *
 * Note: "tooltip" doesn't equal to "tippy". "tooltip" means a auto-popup content, it just uses tippy as the implementation.
 *
 * @param target {HTMLElement}
 * @param content {null|string}
 * @returns {null|tippy}
 */
function attachTooltip(target, content = null) {
  content = content ?? getTooltipContent(target);
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
 * Creating tooltip tippy instance is expensive, so we only create it when the user hovers over the element
 * According to https://www.w3.org/TR/DOM-Level-3-Events/#events-mouseevent-event-order , mouseover event is fired before mouseenter event
 * Some old browsers like Pale Moon doesn't support "mouseenter(capture)"
 * The tippy by default uses "mouseenter" event to show, so we use "mouseover" event to switch to tippy
 * @param e {Event}
 */
function lazyTooltipOnMouseHover(e) {
  e.target.removeEventListener('mouseover', lazyTooltipOnMouseHover, true);
  attachTooltip(this);
}

function getTooltipContent(target) {
  // prefer to always use the "[data-tooltip-content]" attribute
  // for backward compatibility, we also support the ".tooltip[data-content]" attribute
  // in next PR, refactor all the ".tooltip[data-content]" to "[data-tooltip-content]"
  let content = target.getAttribute('data-tooltip-content');
  if (!content && target.classList.contains('tooltip')) {
    content = target.getAttribute('data-content');
  }
  return content;
}

/**
 * Activate the tooltip for all children elements
 * And if the element has no aria-label, use the tooltip content as aria-label
 * @param target {HTMLElement}
 */
function attachChildrenLazyTooltip(target) {
  // the selector must match the logic in getTippyTooltipContent
  for (const el of target.querySelectorAll('[data-tooltip-content], .tooltip[data-content]')) {
    el.addEventListener('mouseover', lazyTooltipOnMouseHover, true);

    // meanwhile, if the element has no aria-label, use the tooltip content as aria-label
    if (!el.hasAttribute('aria-label')) {
      const content = getTooltipContent(el);
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
        // mainly for Vue components and AJAX rendered elements
        for (const el of mutation.addedNodes) {
          // handle all "tooltip" elements in added nodes which have 'querySelectorAll' method, skip non-related nodes (eg: "#text")
          if ('querySelectorAll' in el) {
            attachChildrenLazyTooltip(el);
          }
        }
      } else if (mutation.type === 'attributes') {
        // sync the tooltip content if the attributes change
        attachTooltip(mutation.target);
      }
    }
  });
  observer.observe(document, {
    subtree: true,
    childList: true,
    attributeFilter: ['data-tooltip-content', 'data-content'],
  });

  attachChildrenLazyTooltip(document.documentElement);
}

export function showTemporaryTooltip(target, content) {
  const tippy = target._tippy ?? attachTooltip(target, content);
  tippy.setContent(content);
  if (!tippy.state.isShown) tippy.show();
  tippy.setProps({
    onHidden: (tippy) => {
      // reset the default tooltip content, if no default, then this temporary tooltip could be destroyed
      if (!attachTooltip(target)) {
        tippy.destroy();
      }
    },
  });
}
