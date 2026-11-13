import tippy, {followCursor} from 'tippy.js';
import {isDocumentFragmentOrElementNode, isElemVisible} from '../utils/dom.ts';
import type {Content, Instance, Placement, Props} from 'tippy.js';
import {html, htmlEscape} from '../utils/html.ts';
import {stripTags} from '../utils.ts';

type TippyOpts = {
  role?: string,
  theme?: 'default' | 'tooltip' | 'menu' | 'box-with-header' | 'bare' | 'dropdown',
  limitSizeToViewport?: {horizontal?: boolean, vertical?: boolean}, // cap to the viewport and scroll the content
} & Partial<Props>;

type PopperModifier = NonNullable<NonNullable<Props['popperOptions']>['modifiers']>[number];

const visibleInstances = new Set<Instance>();

const arrowSvg = html`<svg width="16" height="7"><path d="m0 7 8-7 8 7Z" class="tippy-svg-arrow-outer"/><path d="m0 8 8-7 8 7Z" class="tippy-svg-arrow-inner"/></svg>`;

function arrowPadding({placement, reference}: {placement: Placement, reference: {width: number, height: number}}): number {
  const isVertical = placement.startsWith('left') || placement.startsWith('right');
  const referenceLength = isVertical ? reference.height : reference.width;
  return Math.max(0, Math.min(3, referenceLength / 2 - 8));
}

const viewportPadding = 8;

export function availableSizeForPlacement(referenceRect: DOMRect, placement: string, offset: number): {width: number, height: number} {
  const gap = Math.abs(offset) + viewportPadding;
  const spanWidth = window.innerWidth - viewportPadding * 2;
  const spanHeight = window.innerHeight - viewportPadding * 2;
  const side = placement.split('-')[0];
  if (side === 'top') return {width: spanWidth, height: referenceRect.top - gap};
  if (side === 'bottom') return {width: spanWidth, height: window.innerHeight - referenceRect.bottom - gap};
  if (side === 'left') return {width: referenceRect.left - gap, height: spanHeight};
  return {width: window.innerWidth - referenceRect.right - gap, height: spanHeight};
}

function sizeModifier(limit: {horizontal?: boolean, vertical?: boolean}): PopperModifier {
  return {
    name: 'tippyLimitSize',
    enabled: true,
    phase: 'beforeWrite',
    requires: ['computeStyles'],
    fn({state}) {
      const referenceRect = state.elements.reference.getBoundingClientRect();
      const availableFor = (placement: Placement) => {
        const offset = state.modifiersData.offset?.[placement];
        const isVertical = placement.startsWith('top') || placement.startsWith('bottom');
        return availableSizeForPlacement(referenceRect, placement, (isVertical ? offset?.y : offset?.x) ?? 0);
      };
      const available = availableFor(state.placement);
      const opposite = state.placement.replace(/^(top|bottom)/, (side) => side === 'top' ? 'bottom' : 'top') as Placement;
      if (limit.vertical && state.rects.popper.height > available.height && availableFor(opposite).height > available.height) { // Popper keeps the preferred side when neither fits.
        state.placement = opposite;
        state.modifiersData.flip._skip = true;
        state.reset = true;
        return;
      }
      const {style} = state.elements.popper;
      if (limit.horizontal) style.setProperty('--tippy-max-width', `${Math.max(0, Math.floor(available.width))}px`);
      if (limit.vertical) style.setProperty('--tippy-max-height', `${Math.max(0, Math.floor(available.height))}px`);
    },
  };
}

function isMenu(instance: Instance): boolean {
  return instance.props.role === 'menu' && instance.props.theme === 'menu'; // role defaults to "menu" for every non-tooltip popup
}

function focusMenuItem(instance: Instance, delta: number) {
  const items = [...instance.popper.querySelectorAll<HTMLElement>('.item:not(.disabled)')].filter(isElemVisible);
  if (!items.length) return;
  const current = items.indexOf(document.activeElement as HTMLElement);
  const next = current === -1 ? (delta > 0 ? 0 : items.length - 1) : (current + delta + items.length) % items.length;
  items[next].focus();
}

document.addEventListener('keydown', (e: KeyboardEvent) => {
  if (e.isComposing) return;
  const menuInstance = [...visibleInstances].findLast(isMenu);
  if (!menuInstance) return;
  const focused = document.activeElement as HTMLElement;
  const inMenu = menuInstance.popper.contains(focused);
  if (!inMenu && focused !== menuInstance.reference && focused !== document.body) return; // body: macOS Safari and Firefox do not focus clicked buttons
  if (e.key === 'Escape') {
    menuInstance.hide();
  } else if (['ArrowDown', 'ArrowUp', 'ArrowRight', 'ArrowLeft'].includes(e.key)) {
    focusMenuItem(menuInstance, e.key === 'ArrowDown' || e.key === 'ArrowRight' ? 1 : -1);
  } else if ((e.key === 'Enter' || e.key === ' ') && inMenu) {
    focused.click();
  } else {
    return;
  }
  e.preventDefault();
});

export function createTippy(target: Element, opts: TippyOpts = {}): Instance {
  const {onHide, onHidden, onShow, onDestroy, role, theme, arrow, limitSizeToViewport, ...other} = opts;
  const resolvedTheme = theme || role || 'default';
  const resolvedArrow = arrow ?? (resolvedTheme === 'bare' ? false : arrowSvg);

  const instance: Instance = tippy(target, {
    appendTo: document.body,
    animation: false,
    allowHTML: false,
    hideOnClick: false,
    interactiveBorder: 20,
    ignoreAttributes: true,
    maxWidth: 500,
    onHide: (instance: Instance) => {
      visibleInstances.delete(instance);
      if (isMenu(instance) && instance.popper.contains(document.activeElement)) (instance.reference as HTMLElement).focus();
      return onHide?.(instance);
    },
    onHidden: (instance: Instance) => {
      for (const el of instance.popper.querySelectorAll('[data-tooltip-content]')) {
        el._tippy?.clearDelayTimeouts();
        el._tippy?.hide();
      }
      return onHidden?.(instance);
    },
    onDestroy: (instance: Instance) => {
      visibleInstances.delete(instance);
      return onDestroy?.(instance);
    },
    onShow: (instance: Instance) => {
      for (const visibleInstance of visibleInstances) {
        if (visibleInstance.props.role === 'tooltip') {
          visibleInstance.hide();
        }
      }
      visibleInstances.add(instance);
      if (!['tooltip', 'presentation'].includes(instance.props.role)) target.setAttribute('aria-controls', instance.popper.id);
      if (limitSizeToViewport) instance.popper.style.removeProperty('--tippy-max-height'); // flip by the uncapped height
      if (isMenu(instance)) {
        for (const item of instance.popper.querySelectorAll<HTMLElement>('.item')) item.tabIndex = -1;
      }
      return onShow?.(instance);
    },
    arrow: resolvedArrow,
    popperOptions: {modifiers: [
      {name: 'arrow', options: {padding: arrowPadding}},
      ...limitSizeToViewport ? [sizeModifier(limitSizeToViewport)] : [],
    ]},
    role: role || 'menu',
    theme: resolvedTheme,
    offset: [0, resolvedArrow ? 10 : 6],
    plugins: [followCursor],
    ...other,
  } satisfies Partial<Props>);

  if (instance.props.role === 'menu') {
    target.setAttribute('aria-haspopup', 'true');
  }
  if (limitSizeToViewport) instance.popper.setAttribute('data-tippy-limit-size', '');

  return instance;
}

function attachTooltip(target: Element, content: Content | null = null): Instance | null {
  switchTitleToTooltip(target);

  content = content ?? target.getAttribute('data-tooltip-content');
  if (!content) return null;

  let allowHTML = target.getAttribute('data-tooltip-render') === 'html';
  if (!allowHTML && typeof content === 'string') {
    content = htmlEscape(content).split('\n').join(html`<br>`);
    allowHTML = true;
  }

  const props: TippyOpts = {
    content,
    delay: 100,
    role: 'tooltip',
    theme: 'tooltip',
    hideOnClick: !target.hasAttribute('data-clipboard-target'),
    allowHTML,
    placement: target.getAttribute('data-tooltip-placement') as Placement || 'top',
    followCursor: target.getAttribute('data-tooltip-follow-cursor') as Props['followCursor'] || false,
    ...((target.getAttribute('data-tooltip-interactive') === 'true') && {interactive: true, aria: {content: 'describedby', expanded: false}}),
  };

  if (!target._tippy) {
    createTippy(target, props);
  } else {
    target._tippy.setProps(props);
  }
  return target._tippy;
}

function switchTitleToTooltip(target: Element): void {
  const title = target.getAttribute('title');
  if (title) {
    target.setAttribute('data-tooltip-content', title);
    target.setAttribute('aria-label', title);
    target.setAttribute('title', '');
  }
}

function lazyTooltipOnMouseHover(e: Event): void {
  const el = e.currentTarget as HTMLElement;
  el.removeEventListener('mouseover', lazyTooltipOnMouseHover, true);
  const {clientX, clientY} = e as MouseEvent;
  attachTooltip(el)?.reference.dispatchEvent(new MouseEvent('mouseenter', {clientX, clientY})); // Firefox can skip the initial mouseenter.
}

function attachLazyTooltip(el: HTMLElement): void {
  el.addEventListener('mouseover', lazyTooltipOnMouseHover, {capture: true});

  if (!el.hasAttribute('aria-label')) {
    const content = el.getAttribute('data-tooltip-content');
    if (content) {
      const isHtml = el.getAttribute('data-tooltip-render') === 'html';
      let ariaLabelValue = content;
      if (isHtml) ariaLabelValue = stripTags(content).replace(/\s+/g, ' ').trim();
      el.setAttribute('aria-label', ariaLabelValue);
      el.removeAttribute('aria-hidden');
    }
  }
}

function attachChildrenLazyTooltip(target: HTMLElement): void {
  for (const el of target.querySelectorAll<HTMLElement>('[data-tooltip-content]')) {
    attachLazyTooltip(el);
  }
}

export function initGlobalTooltips(): void {
  const observerConnect = (observer: MutationObserver) => observer.observe(document, {
    subtree: true,
    childList: true,
    attributeFilter: ['data-tooltip-content'],
  });
  const observer = new MutationObserver((mutationList, observer) => {
    const pending = observer.takeRecords();
    observer.disconnect();
    for (const mutation of [...mutationList, ...pending]) {
      if (mutation.type === 'childList') {
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
  const popupTippy = target.closest('[data-tippy-root]')?._tippy;
  target = popupTippy ? document.body : target.closest('.dropdown-container')?.querySelector(':scope > .dropdown-trigger') ?? target;
  const refClientRect = (popupTippy?.reference ?? target).getBoundingClientRect();
  const tooltipTippy = target._tippy ?? attachTooltip(target, content);
  tooltipTippy.setContent(content);
  tooltipTippy.setProps({getReferenceClientRect: () => refClientRect});
  if (!tooltipTippy.state.isShown) tooltipTippy.show();

  tooltipTippy.setProps({
    onHidden: (tippy) => {
      if (!attachTooltip(target)) {
        tippy.destroy();
      }
    },
  });

  if (!popupTippy) { // A relocated tooltip receives no mouseout.
    setTimeout(() => {
      if (tooltipTippy.state.isVisible) tooltipTippy.hide();
    }, 1500);
  }
}

export function getAttachedTippyInstance(el: Element): Instance | null {
  return el._tippy ?? null;
}
