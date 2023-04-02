function elementsCall(el, func, ...args) {
  if (typeof el === 'string' || el instanceof String) {
    el = document.querySelectorAll(el);
  }
  if (el instanceof Node) {
    func(el, ...args);
  } else if (el.length !== undefined) {
    // this works for: NodeList, HTMLCollection, Array, jQuery
    for (const e of el) {
      func(e, ...args);
    }
  } else {
    throw new Error('invalid argument to be shown/hidden');
  }
}

/**
 * @param el string (selector), Node, NodeList, HTMLCollection, Array or jQuery
 * @param force force=true to show or force=false to hide, undefined to toggle
 */
function toggleShown(el, force) {
  if (force === true) {
    el.classList.remove('gt-hidden');
  } else if (force === false) {
    el.classList.add('gt-hidden');
  } else if (force === undefined) {
    el.classList.toggle('gt-hidden');
  } else {
    throw new Error('invalid force argument');
  }
}

export function showElem(el) {
  elementsCall(el, toggleShown, true);
}

export function hideElem(el) {
  elementsCall(el, toggleShown, false);
}

export function toggleElem(el, force) {
  elementsCall(el, toggleShown, force);
}

export function onDomReady(cb) {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', cb);
  } else {
    cb();
  }
}
