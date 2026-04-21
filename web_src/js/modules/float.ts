import {computePosition, autoUpdate, flip, shift, offset as offsetMiddleware, arrow as arrowMiddleware, size as sizeMiddleware} from '@floating-ui/dom';
import type {Middleware, Placement, VirtualElement} from '@floating-ui/dom';
import {generateElemId, isDocumentFragmentOrElementNode} from '../utils/dom.ts';
import {html} from '../utils/html.ts';
import {stripTags} from '../utils.ts';

type FloatContent = string | Element | DocumentFragment;
type FloatTheme = 'default' | 'tooltip' | 'menu' | 'box-with-header' | 'bare';
type FollowCursor = false | 'horizontal' | 'vertical';
type Delay = number | [number, number];
type Trigger = 'mouseenter focus' | 'mouseenter' | 'focus' | 'click' | 'focus click' | 'manual';

type FloatProps = {
  content?: FloatContent;
  placement?: Placement;
  role?: string;
  theme?: FloatTheme;
  trigger?: Trigger;
  delay?: Delay;
  offset?: number | [number, number];
  arrow?: boolean;
  interactive?: boolean;
  interactiveBorder?: number;
  hideOnClick?: boolean;
  allowHTML?: boolean;
  maxWidth?: number | string;
  followCursor?: FollowCursor;
  showOnCreate?: boolean;
  getReferenceClientRect?: (() => DOMRect) | null;
  onShow?: (instance: FloatInstance) => void | false;
  onHide?: (instance: FloatInstance) => void | false;
  onHidden?: (instance: FloatInstance) => void;
  onDestroy?: (instance: FloatInstance) => void;
};

export type FloatInstance = {
  float: HTMLElement;
  props: FloatProps;
  state: {isShown: boolean};
  show(): void;
  hide(): void;
  destroy(): void;
  update(): void;
  setContent(content: FloatContent): void;
  setProps(props: Partial<FloatProps>): void;
  enable(): void;
  disable(): void;
};

const defaults = {
  placement: 'top' as Placement,
  theme: 'default' as FloatTheme,
  role: 'menu',
  trigger: 'mouseenter focus' as Trigger,
  arrow: true,
  interactiveBorder: 20,
  maxWidth: 500 as number | string,
  offset: 6,
};

const instances = new WeakMap<Element, FloatInstance>();
const visibleInstances = new Set<FloatInstance>();

let mouseCoords = {x: 0, y: 0};
const cursorListeners = new Set<() => void>();
if (typeof document !== 'undefined') {
  document.addEventListener('pointermove', (e) => {
    mouseCoords = {x: e.clientX, y: e.clientY};
    for (const fn of cursorListeners) fn();
  }, {capture: true});
}

function acquireCursorTracking(fn: () => void): void { cursorListeners.add(fn) }
function releaseCursorTracking(fn: () => void): void { cursorListeners.delete(fn) }

const dismissListeners = new Set<(e: MouseEvent) => void>();
function onGlobalDismiss(e: MouseEvent): void {
  for (const fn of dismissListeners) fn(e);
}
function acquireDismissTracking(fn: (e: MouseEvent) => void): void {
  if (dismissListeners.size === 0) document.addEventListener('mousedown', onGlobalDismiss, true);
  dismissListeners.add(fn);
}
function releaseDismissTracking(fn: (e: MouseEvent) => void): void {
  if (!dismissListeners.delete(fn)) return;
  if (dismissListeners.size === 0) document.removeEventListener('mousedown', onGlobalDismiss, true);
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

function setElementContent(el: HTMLElement, content: FloatContent, allowHTML: boolean): void {
  if (content instanceof Element || content instanceof DocumentFragment) {
    el.replaceChildren(content);
  } else if (allowHTML) {
    el.innerHTML = content;
  } else {
    el.textContent = content;
  }
}

const arrowHtml = html`<svg width="16" height="7"><path d="m0 7 8-7 8 7Z" class="float-arrow-outer"/><path d="m0 8 8-7 8 7Z" class="float-arrow-inner"/></svg>`;

export function createFloat(target: Element, opts: FloatProps = {}): FloatInstance {
  const props: FloatProps = {
    ...defaults,
    arrow: opts.theme === 'bare' ? false : defaults.arrow,
    ...opts,
  };

  const float = document.createElement('div');
  float.className = 'float-box';
  float.id = generateElemId('float-');
  float.setAttribute('data-theme', props.theme ?? defaults.theme);
  float.setAttribute('role', props.role ?? defaults.role);

  const contentEl = document.createElement('div');
  contentEl.className = 'float-content';
  float.append(contentEl);

  let arrowEl: HTMLElement | null = null;
  if (props.arrow) {
    arrowEl = document.createElement('div');
    arrowEl.className = 'float-arrow';
    arrowEl.innerHTML = arrowHtml;
    float.append(arrowEl);
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
  let isCursorOverFloat = false;

  const triggers = parseTriggers(props.trigger ?? defaults.trigger);
  const needsClickHandler = triggers.click || ((triggers.mouse || triggers.focus) && Boolean(props.hideOnClick));
  const floatHoverTracked = Boolean(props.interactive) && triggers.mouse;
  const needsDismissTracking = triggers.click || Boolean(props.interactive);

  const disposers: Array<() => void> = [];
  function listen(el: Element, ev: string, fn: EventListener): void {
    el.addEventListener(ev, fn);
    disposers.push(() => el.removeEventListener(ev, fn));
  }

  const instance = {} as FloatInstance;

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
    if (!float.isConnected) return;
    const offsetOpt = props.offset ?? defaults.offset;
    const middleware: Middleware[] = [
      offsetMiddleware(typeof offsetOpt === 'number' ? (props.arrow ? offsetOpt + 4 : offsetOpt) : {crossAxis: offsetOpt[0], mainAxis: offsetOpt[1]}),
      flip(),
      shift({padding: 8}),
      sizeMiddleware({
        padding: 8,
        apply({availableWidth}) {
          const avail = `${Math.max(0, Math.floor(availableWidth))}px`;
          const cap = typeof props.maxWidth === 'number' ? `${props.maxWidth}px` : (props.maxWidth && props.maxWidth !== 'none' ? props.maxWidth : null);
          float.style.maxWidth = cap ? `min(${cap}, ${avail})` : avail;
        },
      }),
    ];
    if (arrowEl) middleware.push(arrowMiddleware({element: arrowEl, padding: 6}));
    const result = await computePosition(buildReference(), float, {
      strategy: 'absolute',
      placement: props.placement ?? defaults.placement,
      middleware,
    });
    float.style.transform = `translate(${Math.round(result.x)}px, ${Math.round(result.y)}px)`;
    float.setAttribute('data-placement', result.placement);
    if (arrowEl) {
      const side = result.placement.split('-')[0];
      const {x, y} = result.middlewareData.arrow ?? {};
      arrowEl.setAttribute('data-side', side);
      arrowEl.style.left = x === undefined ? '' : `${x}px`;
      arrowEl.style.top = y === undefined ? '' : `${y}px`;
    }
  }

  function startAutoUpdate(): void {
    stopAutoUpdate?.();
    stopAutoUpdate = autoUpdate(buildReference(), float, update);
  }

  function cursorUpdateHandler(): void { update() }

  function onDocDismiss(e: MouseEvent): void {
    if (!isShown) return;
    const t = e.target as Node;
    if (target.contains(t) || float.contains(t)) return;
    doHide();
  }

  function doShow(): void {
    if (isDestroyed || !isEnabled || isShown) return;
    if (props.onShow?.(instance) === false) return;
    isShown = true;
    visibleInstances.add(instance);
    if (props.role === 'tooltip') {
      for (const other of visibleInstances) {
        if (other !== instance && other.props.role === 'tooltip') other.hide();
      }
    }
    document.body.append(float);
    target.setAttribute('aria-controls', float.id);
    if (props.role === 'tooltip') target.setAttribute('aria-describedby', float.id);
    startAutoUpdate();
    if (props.followCursor) acquireCursorTracking(cursorUpdateHandler);
    if (needsDismissTracking) acquireDismissTracking(onDocDismiss);
  }

  function doHide(): void {
    if (isDestroyed || !isShown) return;
    if (props.onHide?.(instance) === false) return;
    isShown = false;
    visibleInstances.delete(instance);
    stopAutoUpdate?.();
    stopAutoUpdate = null;
    if (props.followCursor) releaseCursorTracking(cursorUpdateHandler);
    if (needsDismissTracking) releaseDismissTracking(onDocDismiss);
    float.remove();
    target.removeAttribute('aria-controls');
    if (target.getAttribute('aria-describedby') === float.id) target.removeAttribute('aria-describedby');
    props.onHidden?.(instance);
  }

  function scheduleShow(): void {
    clearTimers();
    const d = resolveDelay(props.delay, 'show');
    if (d > 0) showTimer = window.setTimeout(doShow, d);
    else doShow();
  }

  function scheduleHide(): void {
    clearTimers();
    if (props.interactive && isCursorOverFloat) return;
    const d = resolveDelay(props.delay, 'hide');
    if (d > 0) hideTimer = window.setTimeout(doHide, d);
    else doHide();
  }

  let recentFocusAt = 0;
  function onRefFocus(): void {
    recentFocusAt = performance.now();
    scheduleShow();
  }
  function onRefClick(): void {
    if (!isEnabled) return;
    if (!triggers.click) { if (isShown && props.hideOnClick) doHide(); return }
    if (triggers.focus && performance.now() - recentFocusAt < 200) return;
    if (isShown) doHide(); else doShow();
  }

  if (props.interactive && props.interactiveBorder) {
    float.style.setProperty('--float-interactive-border', `${props.interactiveBorder}px`);
    float.setAttribute('data-interactive', 'true');
  }

  if (triggers.mouse) {
    listen(target, 'mouseenter', scheduleShow);
    listen(target, 'mouseleave', scheduleHide);
  }
  if (triggers.focus) {
    listen(target, 'focus', onRefFocus);
    listen(target, 'blur', scheduleHide);
  }
  if (needsClickHandler) listen(target, 'click', onRefClick);
  if (floatHoverTracked) {
    listen(float, 'mouseenter', () => { isCursorOverFloat = true; clearTimers() });
    listen(float, 'mouseleave', () => { isCursorOverFloat = false; scheduleHide() });
  }

  instance.float = float;
  instance.props = props;
  instance.state = {get isShown() { return isShown }};
  instance.show = doShow;
  instance.hide = doHide;
  instance.update = () => { if (isShown) update(); };
  instance.destroy = () => {
    if (isDestroyed) return;
    clearTimers();
    if (isShown) doHide();
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
    if (partial.theme) float.setAttribute('data-theme', partial.theme);
    if (partial.role) float.setAttribute('role', partial.role);
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
  instance.disable = () => { isEnabled = false; if (isShown) doHide(); };

  instances.set(target, instance);
  if (props.role === 'menu') target.setAttribute('aria-haspopup', 'true');
  if (props.showOnCreate) doShow();
  return instance;
}

/** Attach or update a tooltip Float on `target`. Returns null if content is empty. */
function attachTooltip(target: Element, content: FloatContent | null = null): FloatInstance | null {
  switchTitleToTooltip(target);

  content = content ?? target.getAttribute('data-tooltip-content');
  if (!content) return null;

  const hasClipboardTarget = target.hasAttribute('data-clipboard-target');
  const hideOnClick = !hasClipboardTarget;
  const placement = (target.getAttribute('data-tooltip-placement') as Placement) || 'top-start';
  const interactiveAttr = target.getAttribute('data-tooltip-interactive') === 'true';
  const followCursorAttr = target.getAttribute('data-tooltip-follow-cursor') as FollowCursor || false;
  const allowHTML = target.getAttribute('data-tooltip-render') === 'html';

  const props: Partial<FloatProps> = {
    content,
    delay: 100,
    role: 'tooltip',
    theme: 'tooltip',
    hideOnClick,
    allowHTML,
    placement,
    followCursor: followCursorAttr,
    ...(interactiveAttr ? {interactive: true} : {}),
  };

  const existing = instances.get(target);
  if (existing) existing.setProps(props);
  else createFloat(target, props);
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

/** Lazy first-hover init: `mouseover` bubbles and fires before `mouseenter`,
 *  so the real mouseenter listener attached by `attachTooltip` still fires
 *  for the same user hover, respecting the configured delay. */
function lazyTooltipOnMouseHover(this: HTMLElement, e: Event): void {
  (e.target as HTMLElement).removeEventListener('mouseover', lazyTooltipOnMouseHover, {capture: true});
  attachTooltip(this);
}

function attachLazyTooltip(el: HTMLElement): void {
  el.addEventListener('mouseover', lazyTooltipOnMouseHover, {capture: true});

  if (!el.hasAttribute('aria-label')) {
    const content = el.getAttribute('data-tooltip-content');
    if (content) {
      const isHtml = el.getAttribute('data-tooltip-render') === 'html';
      el.setAttribute('aria-label', isHtml ? stripTags(content).replace(/\s+/g, ' ').trim() : content);
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

export function showTemporaryTooltip(target: Element, content: FloatContent): void {
  let refClientRect: DOMRect | undefined;
  const popupRoot = target.closest<HTMLElement>('.float-box');
  const popupId = popupRoot?.id;
  if (popupId) {
    target = document.body;
    refClientRect = document.querySelector(`[aria-controls="${CSS.escape(popupId)}"]`)?.getBoundingClientRect();
    refClientRect = refClientRect ?? new DOMRect(0, 0, 0, 0);
  } else {
    target = target.closest('.ui.dropdown') ?? target;
    refClientRect = target.getBoundingClientRect();
  }
  const inst = instances.get(target) ?? attachTooltip(target, content);
  if (!inst) return;
  inst.setContent(content);
  inst.setProps({getReferenceClientRect: () => refClientRect});
  if (!inst.state.isShown) inst.show();

  inst.setProps({
    onHidden: (i) => {
      if (!attachTooltip(target)) i.destroy();
    },
  });

  if (!popupId) {
    setTimeout(() => { if (inst.state.isShown) inst.hide(); }, 1500);
  }
}

export function getFloat(el: Element): FloatInstance | null {
  return instances.get(el) ?? null;
}
