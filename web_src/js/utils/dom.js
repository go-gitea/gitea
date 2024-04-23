import {debounce} from 'throttle-debounce';

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
    el.classList.remove('tw-hidden');
  } else if (force === false) {
    el.classList.add('tw-hidden');
  } else if (force === undefined) {
    el.classList.toggle('tw-hidden');
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

export function isElemHidden(el) {
  const res = [];
  elementsCall(el, (e) => res.push(e.classList.contains('tw-hidden')));
  if (res.length > 1) throw new Error(`isElemHidden doesn't work for multiple elements`);
  return res[0];
}

function applyElemsCallback(elems, fn) {
  if (fn) {
    for (const el of elems) {
      fn(el);
    }
  }
  return elems;
}

export function queryElemSiblings(el, selector = '*', fn) {
  return applyElemsCallback(Array.from(el.parentNode.children).filter((child) => child !== el && child.matches(selector)), fn);
}

// it works like jQuery.children: only the direct children are selected
export function queryElemChildren(parent, selector = '*', fn) {
  return applyElemsCallback(parent.querySelectorAll(`:scope > ${selector}`), fn);
}

export function queryElems(selector, fn) {
  return applyElemsCallback(document.querySelectorAll(selector), fn);
}

export function onDomReady(cb) {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', cb);
  } else {
    cb();
  }
}

// checks whether an element is owned by the current document, and whether it is a document fragment or element node
// if it is, it means it is a "normal" element managed by us, which can be modified safely.
export function isDocumentFragmentOrElementNode(el) {
  try {
    return el.ownerDocument === document && el.nodeType === Node.ELEMENT_NODE || el.nodeType === Node.DOCUMENT_FRAGMENT_NODE;
  } catch {
    // in case the el is not in the same origin, then the access to nodeType would fail
    return false;
  }
}

// autosize a textarea to fit content. Based on
// https://github.com/github/textarea-autosize
// ---------------------------------------------------------------------
// Copyright (c) 2018 GitHub, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to
// permit persons to whom the Software is furnished to do so, subject to
// the following conditions:
//
// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.
// ---------------------------------------------------------------------
export function autosize(textarea, {viewportMarginBottom = 0} = {}) {
  let isUserResized = false;
  // lastStyleHeight and initialStyleHeight are CSS values like '100px'
  let lastMouseX, lastMouseY, lastStyleHeight, initialStyleHeight;

  function onUserResize(event) {
    if (isUserResized) return;
    if (lastMouseX !== event.clientX || lastMouseY !== event.clientY) {
      const newStyleHeight = textarea.style.height;
      if (lastStyleHeight && lastStyleHeight !== newStyleHeight) {
        isUserResized = true;
      }
      lastStyleHeight = newStyleHeight;
    }

    lastMouseX = event.clientX;
    lastMouseY = event.clientY;
  }

  function overflowOffset() {
    let offsetTop = 0;
    let el = textarea;

    while (el !== document.body && el !== null) {
      offsetTop += el.offsetTop || 0;
      el = el.offsetParent;
    }

    const top = offsetTop - document.defaultView.scrollY;
    const bottom = document.documentElement.clientHeight - (top + textarea.offsetHeight);
    return {top, bottom};
  }

  function resizeToFit() {
    if (isUserResized) return;
    if (textarea.offsetWidth <= 0 && textarea.offsetHeight <= 0) return;

    try {
      const {top, bottom} = overflowOffset();
      const isOutOfViewport = top < 0 || bottom < 0;

      const computedStyle = getComputedStyle(textarea);
      const topBorderWidth = parseFloat(computedStyle.borderTopWidth);
      const bottomBorderWidth = parseFloat(computedStyle.borderBottomWidth);
      const isBorderBox = computedStyle.boxSizing === 'border-box';
      const borderAddOn = isBorderBox ? topBorderWidth + bottomBorderWidth : 0;

      const adjustedViewportMarginBottom = bottom < viewportMarginBottom ? bottom : viewportMarginBottom;
      const curHeight = parseFloat(computedStyle.height);
      const maxHeight = curHeight + bottom - adjustedViewportMarginBottom;

      textarea.style.height = 'auto';
      let newHeight = textarea.scrollHeight + borderAddOn;

      if (isOutOfViewport) {
        // it is already out of the viewport:
        // * if the textarea is expanding: do not resize it
        if (newHeight > curHeight) {
          newHeight = curHeight;
        }
        // * if the textarea is shrinking, shrink line by line (just use the
        //   scrollHeight). do not apply max-height limit, otherwise the page
        //   flickers and the textarea jumps
      } else {
        // * if it is in the viewport, apply the max-height limit
        newHeight = Math.min(maxHeight, newHeight);
      }

      textarea.style.height = `${newHeight}px`;
      lastStyleHeight = textarea.style.height;
    } finally {
      // ensure that the textarea is fully scrolled to the end, when the cursor
      // is at the end during an input event
      if (textarea.selectionStart === textarea.selectionEnd &&
          textarea.selectionStart === textarea.value.length) {
        textarea.scrollTop = textarea.scrollHeight;
      }
    }
  }

  function onFormReset() {
    isUserResized = false;
    if (initialStyleHeight !== undefined) {
      textarea.style.height = initialStyleHeight;
    } else {
      textarea.style.removeProperty('height');
    }
  }

  textarea.addEventListener('mousemove', onUserResize);
  textarea.addEventListener('input', resizeToFit);
  textarea.form?.addEventListener('reset', onFormReset);
  initialStyleHeight = textarea.style.height ?? undefined;
  if (textarea.value) resizeToFit();

  return {
    resizeToFit,
    destroy() {
      textarea.removeEventListener('mousemove', onUserResize);
      textarea.removeEventListener('input', resizeToFit);
      textarea.form?.removeEventListener('reset', onFormReset);
    },
  };
}

export function onInputDebounce(fn) {
  return debounce(300, fn);
}

// Set the `src` attribute on an element and returns a promise that resolves once the element
// has loaded or errored. Suitable for all elements mention in:
// https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/load_event
export function loadElem(el, src) {
  return new Promise((resolve) => {
    el.addEventListener('load', () => resolve(true), {once: true});
    el.addEventListener('error', () => resolve(false), {once: true});
    el.src = src;
  });
}

// some browsers like PaleMoon don't have "SubmitEvent" support, so polyfill it by a tricky method: use the last clicked button as submitter
// it can't use other transparent polyfill patches because PaleMoon also doesn't support "addEventListener(capture)"
const needSubmitEventPolyfill = typeof SubmitEvent === 'undefined';

export function submitEventSubmitter(e) {
  e = e.originalEvent ?? e; // if the event is wrapped by jQuery, use "originalEvent", otherwise, use the event itself
  return needSubmitEventPolyfill ? (e.target._submitter || null) : e.submitter;
}

function submitEventPolyfillListener(e) {
  const form = e.target.closest('form');
  if (!form) return;
  form._submitter = e.target.closest('button:not([type]), button[type="submit"], input[type="submit"]');
}

export function initSubmitEventPolyfill() {
  if (!needSubmitEventPolyfill) return;
  console.warn(`This browser doesn't have "SubmitEvent" support, use a tricky method to polyfill`);
  document.body.addEventListener('click', submitEventPolyfillListener);
  document.body.addEventListener('focus', submitEventPolyfillListener);
}

/**
 * Check if an element is visible, equivalent to jQuery's `:visible` pseudo.
 * Note: This function doesn't account for all possible visibility scenarios.
 * @param {HTMLElement} element The element to check.
 * @returns {boolean} True if the element is visible.
 */
export function isElemVisible(element) {
  if (!element) return false;

  return Boolean(element.offsetWidth || element.offsetHeight || element.getClientRects().length);
}

// extract text and images from "paste" event
export function getPastedContent(e) {
  const images = [];
  for (const item of e.clipboardData?.items ?? []) {
    if (item.type?.startsWith('image/')) {
      images.push(item.getAsFile());
    }
  }
  const text = e.clipboardData?.getData?.('text') ?? '';
  return {text, images};
}

// replace selected text in a textarea while preserving editor history, e.g. CTRL-Z works after this
export function replaceTextareaSelection(textarea, text) {
  const before = textarea.value.slice(0, textarea.selectionStart ?? undefined);
  const after = textarea.value.slice(textarea.selectionEnd ?? undefined);
  let success = true;

  textarea.contentEditable = 'true';
  try {
    success = document.execCommand('insertText', false, text);
  } catch {
    success = false;
  }
  textarea.contentEditable = 'false';

  if (success && !textarea.value.slice(0, textarea.selectionStart ?? undefined).endsWith(text)) {
    success = false;
  }

  if (!success) {
    textarea.value = `${before}${text}${after}`;
    textarea.dispatchEvent(new CustomEvent('change', {bubbles: true, cancelable: true}));
  }
}
