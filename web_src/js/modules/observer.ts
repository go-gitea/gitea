import {isDocumentFragmentOrElementNode} from '../utils/dom.ts';

type DirElement = HTMLInputElement | HTMLTextAreaElement;

// for performance considerations, it only uses performant syntax
function attachDirAuto(el: Partial<DirElement>) {
  if (el.type !== 'hidden' &&
      el.type !== 'checkbox' &&
      el.type !== 'radio' &&
      el.type !== 'range' &&
      el.type !== 'color') {
    el.dir = 'auto';
  }
}

type GlobalInitFunc<T extends HTMLElement> = (el: T) => void | Promise<void>;
const globalInitFuncs: Record<string, GlobalInitFunc<HTMLElement>> = {};
function attachGlobalInit(el: HTMLElement) {
  const initFunc = el.getAttribute('data-global-init');
  const func = globalInitFuncs[initFunc];
  if (!func) throw new Error(`Global init function "${initFunc}" not found`);
  func(el);
}

type GlobalEventFunc<T extends HTMLElement, E extends Event> = (el: T, e: E) => (void | Promise<void>);
const globalEventFuncs: Record<string, GlobalEventFunc<HTMLElement, Event>> = {};
export function registerGlobalEventFunc<T extends HTMLElement, E extends Event>(event: string, name: string, func: GlobalEventFunc<T, E>) {
  globalEventFuncs[`${event}:${name}`] = func as any;
}

type SelectorHandler = {
  selector: string,
  handler: (el: HTMLElement) => void,
};

const selectorHandlers: SelectorHandler[] = [
  {selector: 'input, textarea', handler: attachDirAuto},
  {selector: '[data-global-init]', handler: attachGlobalInit},
];

export function observeAddedElement(selector: string, handler: (el: HTMLElement) => void) {
  selectorHandlers.push({selector, handler});
  const docNodes = document.querySelectorAll<HTMLElement>(selector);
  for (const el of docNodes) {
    handler(el);
  }
}

export function initAddedElementObserver(): void {
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
          const children = addedNode.querySelectorAll<HTMLElement>(selector);
          for (const el of children) {
            handler(el);
          }
        }
      }
    }
  });

  for (const {selector, handler} of selectorHandlers) {
    const docNodes = document.querySelectorAll<HTMLElement>(selector);
    for (const el of docNodes) {
      handler(el);
    }
  }

  observer.observe(document, {subtree: true, childList: true});

  document.addEventListener('click', (e) => {
    const elem = (e.target as HTMLElement).closest<HTMLElement>('[data-global-click]');
    if (!elem) return;
    const funcName = elem.getAttribute('data-global-click');
    const func = globalEventFuncs[`click:${funcName}`];
    if (!func) throw new Error(`Global event function "click:${funcName}" not found`);
    func(elem, e);
  });
}
