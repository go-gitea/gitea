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

export function isElemHidden(el) {
  const res = [];
  elementsCall(el, (e) => res.push(e.classList.contains('gt-hidden')));
  if (res.length > 1) throw new Error(`isElemHidden doesn't work for multiple elements`);
  return res[0];
}

export function onDomReady(cb) {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', cb);
  } else {
    cb();
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
    }
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
