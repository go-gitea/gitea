function getComputedStyleProperty(el, prop) {
  const cs = el ? window.getComputedStyle(el) : null;
  return cs ? cs[prop] : null;
}

function isShown(el) {
  return getComputedStyleProperty(el, 'display') !== 'none';
}

function assertShown(el, expectShown) {
  if (window.config.runModeIsProd) return;

  // to help developers to catch display bugs, this assertion can be removed after next release cycle or if it has been proved that there is no bug.
  if (expectShown && !isShown(el)) {
    throw new Error('element is hidden but should be shown');
  } else if (!expectShown && isShown(el)) {
    throw new Error('element is shown but should be hidden');
  }
}

function elementsCall(el, func, ...args) {
  if (el instanceof String) {
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

function toggleShown(el, force) {
  if (force === true) {
    el.classList.remove('gt-hidden');
    assertShown(el, true);
  } else if (force === false) {
    el.classList.add('gt-hidden');
    assertShown(el, false);
  } else if (force === undefined) {
    const wasShown = window.config.runModeIsProd ? undefined : isShown(el);
    el.classList.toggle('gt-hidden');
    if (wasShown !== undefined) {
      assertShown(el, !wasShown);
    }
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
