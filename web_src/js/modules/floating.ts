import {
  computePosition, autoUpdate, flip, shift, offset as offsetMiddleware, arrow as arrowMiddleware, size as sizeMiddleware,
} from '@floating-ui/dom';
import type {Middleware, Placement, VirtualElement} from '@floating-ui/dom';
import {generateElemId, isDocumentFragmentOrElementNode, isElemVisible} from '../utils/dom.ts';
import {html, htmlEscape} from '../utils/html.ts';
import {stripTags} from '../utils.ts';

type FloatingContent = string | Element | DocumentFragment;
type FloatingTheme = 'default' | 'tooltip' | 'menu' | 'box-with-header' | 'bare';
type FollowCursor = false | 'horizontal' | 'vertical';
type Delay = number | [number, number];
type Trigger = 'mouseenter focus' | 'mouseenter' | 'focus' | 'click' | 'focus click' | 'manual';

type FloatingElementProps = {
  content?: FloatingContent;
  placement?: Placement;
  role?: string;
  theme?: FloatingTheme;
  trigger?: Trigger;
  delay?: Delay;
  offset?: number | [number, number];
  arrow?: boolean;
  interactive?: boolean;
  interactiveBorder?: number;
  hideOnClick?: boolean;
  allowHTML?: boolean;
  maxWidth?: number | string;
  limitSizeToViewport?: {horizontal?: boolean, vertical?: boolean};
  followCursor?: FollowCursor;
  showOnCreate?: boolean;
  getReferenceClientRect?: (() => DOMRect) | null;
  onShow?: (instance: FloatingElement) => void | false;
  onHide?: (instance: FloatingElement) => void | false;
  onHidden?: (instance: FloatingElement) => void;
  onDestroy?: (instance: FloatingElement) => void;
};

export type FloatingElement = {
  reference: Element;
  element: HTMLElement;
  props: FloatingElementProps;
  state: {isShown: boolean};
  show(): void;
  hide(): void;
  destroy(): void;
  update(): void;
  setContent(content: FloatingContent): void;
  setProps(props: Partial<FloatingElementProps>): void;
  enable(): void;
  disable(): void;
};

const defaults = {
  placement: 'top' as Placement,
  theme: 'default' as FloatingTheme,
  role: 'menu',
  trigger: 'mouseenter focus' as Trigger,
  arrow: true,
  interactiveBorder: 20,
  maxWidth: 500 as number | string,
  offset: 6,
};

const instances = new WeakMap<Element, FloatingElement>();
const visibleInstances = new Set<FloatingElement>();

function isMenu(instance: FloatingElement): boolean {
  return instance.props.role === 'menu' && instance.props.theme === 'menu';
}

function focusMenuItem(instance: FloatingElement, delta: number): void {
  const items = [...instance.element.querySelectorAll<HTMLElement>('.item:not(.disabled)')].filter(isElemVisible);
  if (!items.length) return;
  const current = items.indexOf(document.activeElement as HTMLElement);
  const next = current === -1 ? (delta > 0 ? 0 : items.length - 1) : (current + delta + items.length) % items.length;
  items[next].focus();
}

let mouseCoords = {x: 0, y: 0};
const cursorListeners = new Set<() => void>();
if (typeof document !== 'undefined') {
  document.addEventListener('pointermove', (e) => {
    mouseCoords = {x: e.clientX, y: e.clientY};
    for (const fn of cursorListeners) fn();
  }, {capture: true});
  document.addEventListener('keydown', (e: KeyboardEvent) => {
    if (e.isComposing) return;
    const menuInstance = [...visibleInstances].findLast(isMenu);
    if (!menuInstance) return;
    const focused = document.activeElement as HTMLElement;
    const inMenu = menuInstance.element.contains(focused);
    if (!inMenu && focused !== menuInstance.reference && focused !== document.body) return;
    if (e.key === 'Escape') {
      menuInstance.hide();
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      focusMenuItem(menuInstance, e.key === 'ArrowDown' ? 1 : -1);
    } else if ((e.key === 'Enter' || e.key === ' ') && inMenu) {
      focused.click();
    } else {
      return;
    }
    e.preventDefault();
  });
}

function acquireCursorTracking(fn: () => void): void { cursorListeners.add(fn) }
function releaseCursorTracking(fn: () => void): void { cursorListeners.delete(fn) }

function isCursorWithinRect(rect: Pick<DOMRect, 'left' | 'right' | 'top' | 'bottom'>, border = 0): boolean {
  return mouseCoords.x >= rect.left - border && mouseCoords.x <= rect.right + border &&
    mouseCoords.y >= rect.top - border && mouseCoords.y <= rect.bottom + border;
}

const dismissListeners = new Set<(e: MouseEvent) => void>();
function onGlobalDismiss(e: MouseEvent): void {
  for (const fn of dismissListeners) fn(e);
}
function acquireDismissTracking(fn: (e: MouseEvent) => void): void {
  if (dismissListeners.size === 0) document.addEventListener('mousedown', onGlobalDismiss, {capture: true});
  dismissListeners.add(fn);
}
function releaseDismissTracking(fn: (e: MouseEvent) => void): void {
  if (!dismissListeners.delete(fn)) return;
  if (dismissListeners.size === 0) document.removeEventListener('mousedown', onGlobalDismiss, {capture: true});
}

function resolveDelay(delay: Delay | undefined, kind: 'show' | 'hide'): number {
  if (delay === undefined) return 0;
  if (typeof delay === 'number') return delay;
  return kind === 'show' ? delay[0] : delay[1];
}

function parseTriggers(trigger: Trigger): {mouse: boolean; focus: boolean; click: boolean} {
  if (trigger === 'manual') return {mouse: false, focus: false, click: false};
  const parts = new Set(trigger.split(/\s+/));
  return {mouse: parts.has('mouseenter'), focus: parts.has('focus'), click: parts.has('click')};
}

function setElementContent(el: HTMLElement, content: FloatingContent, allowHTML: boolean): void {
  if (content instanceof Element || content instanceof DocumentFragment) {
    el.replaceChildren(content);
  } else if (allowHTML) {
    el.innerHTML = content;
  } else {
    el.textContent = content;
  }
}

const arrowHtml = html`<svg class="floating-arrow" width="16" height="16" viewBox="0 0 16 16" overflow="visible"><path d="M0,0 H16 L8,7 Z" class="floating-arrow-outer"/><path d="M0,-1 H16 L8,6 Z" class="floating-arrow-inner"/></svg>`;

export function createFloatingElement(target: Element, opts: FloatingElementProps = {}): FloatingElement {
  const theme = opts.theme || opts.role as FloatingTheme || defaults.theme;
  const props: FloatingElementProps = {
    ...defaults,
    theme,
    arrow: theme === 'bare' ? false : defaults.arrow,
    ...opts,
  };

  const el = document.createElement('div');
  el.className = 'floating-box';
  el.id = generateElemId('floating-');
  el.setAttribute('data-theme', props.theme ?? defaults.theme);
  el.setAttribute('role', props.role ?? defaults.role);
  if (props.limitSizeToViewport) el.setAttribute('data-floating-limit-size', '');

  const contentEl = document.createElement('div');
  contentEl.className = 'floating-content';
  el.append(contentEl);

  let arrowEl: SVGSVGElement | null = null;
  if (props.arrow) {
    const tmpl = document.createElement('template');
    tmpl.innerHTML = arrowHtml;
    arrowEl = tmpl.content.firstElementChild as SVGSVGElement;
    el.append(arrowEl);
  }

  if (props.content !== undefined && props.content !== null) {
    setElementContent(contentEl, props.content, Boolean(props.allowHTML));
  }

  let showTimer: number | undefined;
  let hideTimer: number | undefined;
  let stopAutoUpdate: (() => void) | null = null;
  let isDestroyed = false;
  let isEnabled = true;
  let isShown = false;
  let isCursorOverFloating = false;

  const triggers = parseTriggers(props.trigger ?? defaults.trigger);
  const needsClickHandler = triggers.click || ((triggers.mouse || triggers.focus) && Boolean(props.hideOnClick));
  const floatingHoverTracked = Boolean(props.interactive) && triggers.mouse;
  const needsDismissTracking = triggers.click || Boolean(props.hideOnClick);
  const managesExpanded = props.interactive && props.role !== 'tooltip' && !target.hasAttribute('aria-expanded');
  if (managesExpanded) target.setAttribute('aria-expanded', 'false');

  const disposers: Array<() => void> = [];
  function listen(el: Element, ev: string, fn: EventListener): void {
    el.addEventListener(ev, fn);
    disposers.push(() => el.removeEventListener(ev, fn));
  }

  const instance = {} as FloatingElement;

  function clearTimers(): void {
    if (showTimer !== undefined) { clearTimeout(showTimer); showTimer = undefined }
    if (hideTimer !== undefined) { clearTimeout(hideTimer); hideTimer = undefined }
  }

  function buildReference(): Element | VirtualElement {
    const getRect = props.getReferenceClientRect;
    if (getRect) {
      return {getBoundingClientRect: () => getRect(), contextElement: target} satisfies VirtualElement;
    }
    if (props.followCursor) {
      const mode = props.followCursor;
      return {
        getBoundingClientRect: () => {
          const rect = target.getBoundingClientRect();
          const {x, y} = mouseCoords;
          const top = mode === 'horizontal' ? rect.top : y;
          const bottom = mode === 'horizontal' ? rect.bottom : y;
          const left = mode === 'vertical' ? rect.left : x;
          const right = mode === 'vertical' ? rect.right : x;
          return new DOMRect(left, top, right - left, bottom - top);
        },
        contextElement: target,
      } satisfies VirtualElement;
    }
    return target;
  }

  async function update(): Promise<void> {
    if (!el.isConnected) return;
    const offsetOpt = props.offset ?? defaults.offset;
    const middleware: Middleware[] = [
      offsetMiddleware(typeof offsetOpt === 'number' ? (props.arrow ? offsetOpt + 4 : offsetOpt) : {crossAxis: offsetOpt[0], mainAxis: offsetOpt[1]}),
      flip(),
      shift({padding: 8}),
      sizeMiddleware({
        padding: 8,
        apply({availableWidth, availableHeight}) {
          const avail = `${Math.max(0, Math.floor(availableWidth))}px`;
          const cap = typeof props.maxWidth === 'number' ? `${props.maxWidth}px` : (props.maxWidth && props.maxWidth !== 'none' ? props.maxWidth : null);
          el.style.maxWidth = cap ? `min(${cap}, ${avail})` : avail;
          el.style.maxHeight = props.limitSizeToViewport?.vertical ? `${Math.max(0, Math.floor(availableHeight))}px` : '';
        },
      }),
    ];
    if (arrowEl) middleware.push(arrowMiddleware(({placement, rects}) => ({
      element: arrowEl,
      padding: Math.max(0, Math.min(3, (placement.startsWith('left') || placement.startsWith('right') ? rects.reference.height : rects.reference.width) / 2 - 8)),
    })));
    const result = await computePosition(buildReference(), el, {
      strategy: 'absolute',
      placement: props.placement ?? defaults.placement,
      middleware,
    });
    el.style.transform = `translate(${Math.round(result.x)}px, ${Math.round(result.y)}px)`;
    el.setAttribute('data-placement', result.placement);
    if (arrowEl) {
      const side = result.placement.split('-')[0] as 'top' | 'bottom' | 'left' | 'right';
      const {x, y} = result.middlewareData.arrow ?? {};
      const rotation = {top: '', bottom: 'rotate(180deg)', left: 'rotate(-90deg)', right: 'rotate(90deg)'}[side];
      Object.assign(arrowEl.style, {
        left: x === undefined ? '' : `${x}px`,
        right: '',
        top: y === undefined ? '' : `${y}px`,
        bottom: '',
        [side]: '100%',
        transform: rotation,
      });
    }
  }

  function startAutoUpdate(): void {
    stopAutoUpdate?.();
    stopAutoUpdate = autoUpdate(buildReference(), el, update);
  }

  function cursorUpdateHandler(): void { update() }

  function onInteractiveMove(): void {
    const referenceRect = target.getBoundingClientRect();
    const rect = el.getBoundingClientRect();
    const side = (el.getAttribute('data-placement') || props.placement || defaults.placement).split('-')[0];
    if (isCursorWithinRect(referenceRect) || isCursorWithinRect({
      left: side === 'right' ? Math.min(rect.left, referenceRect.right) : rect.left,
      right: side === 'left' ? Math.max(rect.right, referenceRect.left) : rect.right,
      top: side === 'bottom' ? Math.min(rect.top, referenceRect.bottom) : rect.top,
      bottom: side === 'top' ? Math.max(rect.bottom, referenceRect.top) : rect.bottom,
    }, props.interactiveBorder ?? defaults.interactiveBorder)) clearTimers();
    else if (hideTimer === undefined) scheduleHide();
  }

  function onInteractiveLeave(event: Event): void {
    if (!isShown) { scheduleHide(); return }
    const {clientX, clientY} = event as MouseEvent;
    mouseCoords = {x: clientX, y: clientY};
    acquireCursorTracking(onInteractiveMove);
    onInteractiveMove();
  }

  function onDocDismiss(e: MouseEvent): void {
    if (!props.hideOnClick) return;
    const targetNode = e.target as Node;
    if ((triggers.click && target.contains(targetNode)) || (props.interactive && el.contains(targetNode))) return;
    doHide();
  }

  function doShow(): void {
    clearTimers();
    if (isDestroyed || !isEnabled || isShown) return;
    if (props.onShow?.(instance) === false) { releaseDismissTracking(onDocDismiss); return }
    isShown = true;
    visibleInstances.add(instance);
    for (const other of visibleInstances) {
      if (other !== instance && other.props.role === 'tooltip') other.hide();
    }
    if (isMenu(instance)) {
      for (const item of el.querySelectorAll<HTMLElement>('.item')) item.tabIndex = -1;
    }
    document.body.append(el);
    target.setAttribute('aria-controls', el.id);
    if (managesExpanded) target.setAttribute('aria-expanded', 'true');
    if (props.role === 'tooltip') target.setAttribute('aria-describedby', el.id);
    startAutoUpdate();
    if (props.followCursor) acquireCursorTracking(cursorUpdateHandler);
    if (needsDismissTracking) acquireDismissTracking(onDocDismiss);
  }

  function doHide(force = false): void {
    clearTimers();
    if (isDestroyed || !isShown) { releaseDismissTracking(onDocDismiss); return }
    if (props.onHide?.(instance) === false && !force) return;
    isShown = false;
    if (managesExpanded) target.setAttribute('aria-expanded', 'false');
    visibleInstances.delete(instance);
    stopAutoUpdate?.();
    stopAutoUpdate = null;
    releaseCursorTracking(onInteractiveMove);
    isCursorOverFloating = false;
    if (props.followCursor) releaseCursorTracking(cursorUpdateHandler);
    if (needsDismissTracking) releaseDismissTracking(onDocDismiss);
    if (isMenu(instance) && el.contains(document.activeElement)) (target as HTMLElement).focus();
    el.remove();
    if (target.getAttribute('aria-describedby') === el.id) target.removeAttribute('aria-describedby');
    props.onHidden?.(instance);
  }

  function scheduleShow(): void {
    clearTimers();
    if (isDestroyed || !isEnabled) return;
    if (needsDismissTracking) acquireDismissTracking(onDocDismiss);
    const d = resolveDelay(props.delay, 'show');
    if (d > 0) showTimer = window.setTimeout(doShow, d);
    else doShow();
  }

  function scheduleHide(): void {
    clearTimers();
    if (props.interactive && isCursorOverFloating) return;
    const d = resolveDelay(props.delay, 'hide');
    if (d > 0 || props.interactive) hideTimer = window.setTimeout(doHide, d);
    else doHide();
  }

  let wasFocused = false;
  function onRefFocus(): void {
    wasFocused = true;
    scheduleShow();
  }
  function onRefClick(): void {
    if (!isEnabled) return;
    if (!triggers.click) { if (props.hideOnClick) doHide(); return }
    if (triggers.focus && wasFocused) { wasFocused = false; return }
    if (isShown) doHide(); else doShow();
  }

  if (triggers.mouse) {
    listen(target, 'mouseenter', (e) => {
      const {clientX, clientY} = e as MouseEvent;
      mouseCoords = {x: clientX, y: clientY};
      releaseCursorTracking(onInteractiveMove);
      scheduleShow();
    });
    listen(target, 'mouseleave', props.interactive ? onInteractiveLeave : scheduleHide);
  }
  if (triggers.focus) {
    listen(target, 'focus', onRefFocus);
    listen(target, 'blur', (e) => {
      if (props.interactive && el.contains((e as FocusEvent).relatedTarget as Node | null)) return;
      scheduleHide();
    });
  }
  if (needsClickHandler) listen(target, 'click', onRefClick);
  if (floatingHoverTracked) {
    listen(el, 'mouseenter', () => {
      isCursorOverFloating = true;
      releaseCursorTracking(onInteractiveMove);
      clearTimers();
    });
    listen(el, 'mouseleave', (e) => { isCursorOverFloating = false; onInteractiveLeave(e) });
  }

  instance.reference = target;
  instance.element = el;
  instance.props = props;
  instance.state = {get isShown() { return isShown }};
  instance.show = doShow;
  instance.hide = doHide;
  instance.update = () => { if (isShown) update(); };
  instance.destroy = () => {
    if (isDestroyed) return;
    doHide(true);
    isDestroyed = true;
    for (const dispose of disposers) dispose();
    instances.delete(target);
    props.onDestroy?.(instance);
  };
  instance.setContent = (c) => {
    props.content = c;
    setElementContent(contentEl, c, Boolean(props.allowHTML));
    if (isShown) update();
  };
  instance.setProps = (partial) => {
    const wasFollow = Boolean(props.followCursor);
    Object.assign(props, partial);
    if (partial.theme) el.setAttribute('data-theme', partial.theme);
    if (partial.role) el.setAttribute('role', partial.role);
    if ('limitSizeToViewport' in partial) el.toggleAttribute('data-floating-limit-size', Boolean(partial.limitSizeToViewport));
    if (partial.content !== undefined && partial.content !== null) {
      setElementContent(contentEl, partial.content, Boolean(props.allowHTML));
    }
    if (!isShown) return;
    const nowFollow = Boolean(props.followCursor);
    if (wasFollow && !nowFollow) releaseCursorTracking(cursorUpdateHandler);
    else if (!wasFollow && nowFollow) acquireCursorTracking(cursorUpdateHandler);
    const refChanged = 'getReferenceClientRect' in partial || 'followCursor' in partial || 'placement' in partial;
    if (refChanged) startAutoUpdate();
    update();
  };
  instance.enable = () => { isEnabled = true };
  instance.disable = () => { isEnabled = false; doHide() };

  instances.set(target, instance);
  if (props.role === 'menu') target.setAttribute('aria-haspopup', 'true');
  if (props.showOnCreate) doShow();
  return instance;
}

function attachTooltip(target: Element, content: FloatingContent | null = null): FloatingElement | null {
  switchTitleToTooltip(target);

  content = content ?? target.getAttribute('data-tooltip-content');
  if (!content) return null;

  const hasClipboardTarget = target.hasAttribute('data-clipboard-target');
  const hideOnClick = !hasClipboardTarget;
  const placement = (target.getAttribute('data-tooltip-placement') as Placement) || 'top';
  const interactiveAttr = target.getAttribute('data-tooltip-interactive') === 'true';
  const followCursorAttr = target.getAttribute('data-tooltip-follow-cursor') as FollowCursor || false;
  let allowHTML = target.getAttribute('data-tooltip-render') === 'html';
  if (!allowHTML && typeof content === 'string') {
    content = htmlEscape(content).replace(/\n/g, '<br>');
    allowHTML = true;
  }

  const props: Partial<FloatingElementProps> = {
    content,
    delay: 100,
    role: 'tooltip',
    theme: 'tooltip',
    hideOnClick,
    allowHTML,
    placement,
    followCursor: followCursorAttr,
    getReferenceClientRect: null,
    ...(interactiveAttr && {interactive: true}),
  };

  const existing = instances.get(target);
  if (existing) existing.setProps(props);
  else createFloatingElement(target, props);
  return instances.get(target) ?? null;
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
  el.removeEventListener('mouseover', lazyTooltipOnMouseHover, {capture: true});
  if (!attachTooltip(el)) return;
  const {clientX, clientY} = e as MouseEvent;
  el.dispatchEvent(new MouseEvent('mouseenter', {clientX, clientY}));
}

function attachLazyTooltip(el: HTMLElement): void {
  el.addEventListener('mouseover', lazyTooltipOnMouseHover, {capture: true});

  if (!el.hasAttribute('aria-label')) {
    const content = el.getAttribute('data-tooltip-content');
    if (content) {
      const isHtml = el.getAttribute('data-tooltip-render') === 'html';
      el.setAttribute('aria-label', isHtml ? stripTags(content).replace(/\s+/g, ' ').trim() : content);
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
          if (el.hasAttribute('data-tooltip-content')) attachLazyTooltip(el);
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

export function showTemporaryTooltip(target: Element, content: FloatingContent): void {
  let refClientRect: DOMRect | undefined;
  const popupRoot = target.closest<HTMLElement>('.floating-box');
  const popupId = popupRoot?.id;
  if (popupId) {
    target = document.body;
    refClientRect = document.querySelector(`[aria-controls="${CSS.escape(popupId)}"]`)?.getBoundingClientRect();
    refClientRect = refClientRect ?? new DOMRect(0, 0, 0, 0);
  } else {
    target = target.closest('.ui.dropdown') ?? target;
    refClientRect = target.getBoundingClientRect();
  }
  const instance = instances.get(target) ?? attachTooltip(target, content);
  if (!instance) return;
  instance.setContent(content);
  instance.setProps({getReferenceClientRect: () => refClientRect});
  if (!instance.state.isShown) instance.show();

  instance.setProps({
    onHidden: (i) => {
      if (!attachTooltip(target)) i.destroy();
    },
  });

  if (!popupId) {
    setTimeout(() => { if (instance.state.isShown) instance.hide(); }, 1500);
  }
}

export function getFloatingElement(el: Element): FloatingElement | null {
  return instances.get(el) ?? null;
}

type FloatingDelegateProps = Omit<FloatingElementProps, 'content' | 'trigger' | 'getReferenceClientRect'> & {
  target: string;
  content: (el: Element) => FloatingContent;
};

export function delegate(container: Element | string, props: FloatingDelegateProps): () => void {
  const resolved = typeof container === 'string' ? document.querySelector(container) : container;
  if (!resolved) return () => {};
  const root: Element = resolved;
  const {target, content, ...floatingProps} = props;
  const anchor = document.createElement('div');
  anchor.style.position = 'fixed';
  document.body.append(anchor);
  let hovered: Element | null = null;
  const instance = createFloatingElement(anchor, {
    ...floatingProps,
    trigger: 'manual',
    getReferenceClientRect: () => (hovered ?? anchor).getBoundingClientRect(),
  });
  instance.element.style.transition = 'transform 0.1s ease-out';

  function onOver(e: Event): void {
    const el = (e.target as Element).closest(target);
    if (!el || !root.contains(el)) {
      hovered = null;
      instance.hide();
      return;
    }
    hovered = el;
    instance.setContent(content(el));
    if (!instance.state.isShown) instance.show();
  }
  function onLeave(): void {
    hovered = null;
    instance.hide();
  }
  root.addEventListener('mouseover', onOver);
  root.addEventListener('mouseleave', onLeave);

  return () => {
    root.removeEventListener('mouseover', onOver);
    root.removeEventListener('mouseleave', onLeave);
    instance.destroy();
    anchor.remove();
  };
}
