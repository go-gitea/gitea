import tippy, {followCursor} from 'tippy.js';
import {isDocumentFragmentOrElementNode} from '../utils/dom.ts';
import {formatDatetime} from '../utils/time.ts';
import type {Content, Instance, Placement, Props} from 'tippy.js';

type TippyOpts = {
  role?: string,
  theme?: 'default' | 'tooltip' | 'menu' | 'box-with-header' | 'bare',
} & Partial<Props>;

const visibleInstances = new Set<Instance>();
const arrowSvg = `<svg width="16" height="7"><path d="m0 7 8-7 8 7Z" class="tippy-svg-arrow-outer"/><path d="m0 8 8-7 8 7Z" class="tippy-svg-arrow-inner"/></svg>`;

export function createTippy(target: Element, opts: TippyOpts = {}): Instance {
  // the callback functions should be destructured from opts,
  // because we should use our own wrapper functions to handle them, do not let the user override them
  const {onHide, onShow, onDestroy, role, theme, arrow, ...other} = opts;

  const instance: Instance = tippy(target, {
    appendTo: document.body,
    animation: false,
    allowHTML: false,
    hideOnClick: false,
    interactiveBorder: 20,
    ignoreAttributes: true,
    maxWidth: 500, // increase over default 350px
    onHide: (instance: Instance) => {
      visibleInstances.delete(instance);
      return onHide?.(instance);
    },
    onDestroy: (instance: Instance) => {
      visibleInstances.delete(instance);
      return onDestroy?.(instance);
    },
    onShow: (instance: Instance) => {
      // hide other tooltip instances so only one tooltip shows at a time
      for (const visibleInstance of visibleInstances) {
        if (visibleInstance.props.role === 'tooltip') {
          visibleInstance.hide();
        }
      }
      visibleInstances.add(instance);
      return onShow?.(instance);
    },
    arrow: arrow ?? (theme === 'bare' ? false : arrowSvg),
    // HTML role attribute, ideally the default role would be "popover" but it does not exist
    role: role || 'menu',
    // CSS theme, either "default", "tooltip", "menu", "box-with-header" or "bare"
    theme: theme || role || 'default',
    offset: [0, arrow ? 10 : 6],
    plugins: [followCursor],
    ...other,
  } satisfies Partial<Props>);

  if (instance.props.role === 'menu') {
    target.setAttribute('aria-haspopup', 'true');
  }

  return instance;
}

/**
 * Attach a tooltip tippy to the given target element.
 * If the target element already has a tooltip tippy attached, the tooltip will be updated with the new content.
 * If the target element has no content, then no tooltip will be attached, and it returns null.
 *
 * Note: "tooltip" doesn't equal to "tippy". "tooltip" means a auto-popup content, it just uses tippy as the implementation.
 */
function attachTooltip(target: Element, content: Content = null): Instance {
  switchTitleToTooltip(target);

  content = content ?? target.getAttribute('data-tooltip-content');
  if (!content) return null;

  // when element has a clipboard target, we update the tooltip after copy
  // in which case it is undesirable to automatically hide it on click as
  // it would momentarily flash the tooltip out and in.
  const hasClipboardTarget = target.hasAttribute('data-clipboard-target');
  const hideOnClick = !hasClipboardTarget;

  const props: TippyOpts = {
    content,
    delay: 100,
    role: 'tooltip',
    theme: 'tooltip',
    hideOnClick,
    placement: target.getAttribute('data-tooltip-placement') as Placement || 'top-start',
    followCursor: target.getAttribute('data-tooltip-follow-cursor') as Props['followCursor'] || false,
    ...(target.getAttribute('data-tooltip-interactive') === 'true' ? {interactive: true, aria: {content: 'describedby', expanded: false}} : {}),
  };

  if (!target._tippy) {
    createTippy(target, props);
  } else {
    target._tippy.setProps(props);
  }
  return target._tippy;
}

function switchTitleToTooltip(target: Element): void {
  let title = target.getAttribute('title');
  if (title) {
    // apply custom formatting to relative-time's tooltips
    if (target.tagName.toLowerCase() === 'relative-time') {
      const datetime = target.getAttribute('datetime');
      if (datetime) {
        title = formatDatetime(new Date(datetime));
      }
    }
    target.setAttribute('data-tooltip-content', title);
    target.setAttribute('aria-label', title);
    // keep the attribute, in case there are some other "[title]" selectors
    // and to prevent infinite loop with <relative-time> which will re-add
    // title if it is absent
    target.setAttribute('title', '');
  }
}

/**
 * Creating tooltip tippy instance is expensive, so we only create it when the user hovers over the element
 * According to https://www.w3.org/TR/DOM-Level-3-Events/#events-mouseevent-event-order , mouseover event is fired before mouseenter event
 * Some browsers like PaleMoon don't support "addEventListener('mouseenter', capture)"
 * The tippy by default uses "mouseenter" event to show, so we use "mouseover" event to switch to tippy
 */
function lazyTooltipOnMouseHover(this: HTMLElement, e: Event): void {
  e.target.removeEventListener('mouseover', lazyTooltipOnMouseHover, true);
  attachTooltip(this);
}

// Activate the tooltip for current element.
// If the element has no aria-label, use the tooltip content as aria-label.
function attachLazyTooltip(el: HTMLElement): void {
  el.addEventListener('mouseover', lazyTooltipOnMouseHover, {capture: true});

  // meanwhile, if the element has no aria-label, use the tooltip content as aria-label
  if (!el.hasAttribute('aria-label')) {
    const content = el.getAttribute('data-tooltip-content');
    if (content) {
      el.setAttribute('aria-label', content);
    }
  }
}

// Activate the tooltip for all children elements.
function attachChildrenLazyTooltip(target: HTMLElement): void {
  for (const el of target.querySelectorAll<HTMLElement>('[data-tooltip-content]')) {
    attachLazyTooltip(el);
  }
}

export function initGlobalTooltips(): void {
  // use MutationObserver to detect new "data-tooltip-content" elements added to the DOM, or attributes changed
  const observerConnect = (observer: MutationObserver) => observer.observe(document, {
    subtree: true,
    childList: true,
    attributeFilter: ['data-tooltip-content', 'title'],
  });
  const observer = new MutationObserver((mutationList, observer) => {
    const pending = observer.takeRecords();
    observer.disconnect();
    for (const mutation of [...mutationList, ...pending]) {
      if (mutation.type === 'childList') {
        // mainly for Vue components and AJAX rendered elements
        for (const el of mutation.addedNodes as NodeListOf<HTMLElement>) {
          if (!isDocumentFragmentOrElementNode(el)) continue;
          attachChildrenLazyTooltip(el);
          if (el.hasAttribute('data-tooltip-content')) {
            attachLazyTooltip(el);
          }
        }
      } else if (mutation.type === 'attributes') {
        attachTooltip(mutation.target as Element);
      }
    }
    observerConnect(observer);
  });
  observerConnect(observer);

  attachChildrenLazyTooltip(document.documentElement);
}

export function showTemporaryTooltip(target: Element, content: Content): void {
  // if the target is inside a dropdown, the menu will be hidden soon
  // so display the tooltip on the dropdown instead
  target = target.closest('.ui.dropdown') || target;
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
