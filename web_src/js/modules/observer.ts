import {isDocumentFragmentOrElementNode} from '../utils/dom.ts';
import type {Promisable} from 'type-fest';
import type {InitPerformanceTracer} from './init.ts';

let globalSelectorObserverInited = false;

type SelectorHandler = {selector: string, handler: (el: HTMLElement) => void};
const selectorHandlers: SelectorHandler[] = [];

type GlobalEventFunc<T extends HTMLElement, E extends Event> = (el: T, e: E) => Promisable<void>;
const globalEventFuncs: Record<string, GlobalEventFunc<HTMLElement, Event>> = {};

type GlobalInitFunc<T extends HTMLElement> = (el: T) => Promisable<void>;
const globalInitFuncs: Record<string, GlobalInitFunc<HTMLElement>> = {};

// It handles the global events for all `<div data-global-click="onSomeElemClick"></div>` elements.
export function registerGlobalEventFunc<T extends HTMLElement, E extends Event>(event: string, name: string, func: GlobalEventFunc<T, E>) {
  globalEventFuncs[`${event}:${name}`] = func as GlobalEventFunc<HTMLElement, Event>;
}

// It handles the global init functions by a selector, for example:
// > registerGlobalSelectorObserver('.ui.dropdown:not(.custom)', (el) => { initDropdown(el, ...) });
// ATTENTION: For most cases, it's recommended to use registerGlobalInitFunc instead,
// Because this selector-based approach is less efficient and less maintainable.
// But if there are already a lot of elements on many pages, this selector-based approach is more convenient for exiting code.
export function registerGlobalSelectorFunc(selector: string, handler: (el: HTMLElement) => void) {
  selectorHandlers.push({selector, handler});
  // Then initAddedElementObserver will call this handler for all existing elements after all handlers are added.
  // This approach makes the init stage only need to do one "querySelectorAll".
  if (!globalSelectorObserverInited) return;
  for (const el of document.querySelectorAll<HTMLElement>(selector)) {
    handler(el);
  }
}

// It handles the global init functions for all `<div data-global-int="initSomeElem"></div>` elements.
export function registerGlobalInitFunc<T extends HTMLElement>(name: string, handler: GlobalInitFunc<T>) {
  globalInitFuncs[name] = handler as GlobalInitFunc<HTMLElement>;
  // The "global init" functions are managed internally and called by callGlobalInitFunc
  // They must be ready before initGlobalSelectorObserver is called.
  if (globalSelectorObserverInited) throw new Error('registerGlobalInitFunc() must be called before initGlobalSelectorObserver()');
}

function callGlobalInitFunc(el: HTMLElement) {
  const initFunc = el.getAttribute('data-global-init');
  const func = globalInitFuncs[initFunc];
  if (!func) throw new Error(`Global init function "${initFunc}" not found`);

  type GiteaGlobalInitElement = Partial<HTMLElement> & {_giteaGlobalInited: boolean};
  if ((el as GiteaGlobalInitElement)._giteaGlobalInited) throw new Error(`Global init function "${initFunc}" already executed`);
  (el as GiteaGlobalInitElement)._giteaGlobalInited = true;
  func(el);
}

function attachGlobalEvents() {
  // add global "[data-global-click]" event handler
  document.addEventListener('click', (e) => {
    const elem = (e.target as HTMLElement).closest<HTMLElement>('[data-global-click]');
    if (!elem) return;
    const funcName = elem.getAttribute('data-global-click');
    const func = globalEventFuncs[`click:${funcName}`];
    if (!func) throw new Error(`Global event function "click:${funcName}" not found`);
    func(elem, e);
  });
}

export function initGlobalSelectorObserver(perfTracer?: InitPerformanceTracer): void {
  if (globalSelectorObserverInited) throw new Error('initGlobalSelectorObserver() already called');
  globalSelectorObserverInited = true;

  attachGlobalEvents();

  selectorHandlers.push({selector: '[data-global-init]', handler: callGlobalInitFunc});
  const observer = new MutationObserver((mutationList) => {
    const len = mutationList.length;
    for (let i = 0; i < len; i++) {
      const mutation = mutationList[i];
      const len = mutation.addedNodes.length;
      for (let i = 0; i < len; i++) {
        const addedNode = mutation.addedNodes[i] as HTMLElement;
        if (!isDocumentFragmentOrElementNode(addedNode)) continue;

        for (const {selector, handler} of selectorHandlers) {
          if (addedNode.matches(selector)) {
            handler(addedNode);
          }
          for (const el of addedNode.querySelectorAll<HTMLElement>(selector)) {
            handler(el);
          }
        }
      }
    }
  });
  if (perfTracer) {
    for (const {selector, handler} of selectorHandlers) {
      perfTracer.recordCall(`initGlobalSelectorObserver ${selector}`, () => {
        for (const el of document.querySelectorAll<HTMLElement>(selector)) {
          handler(el);
        }
      });
    }
  } else {
    for (const {selector, handler} of selectorHandlers) {
      for (const el of document.querySelectorAll<HTMLElement>(selector)) {
        handler(el);
      }
    }
  }
  observer.observe(document, {subtree: true, childList: true});
}
