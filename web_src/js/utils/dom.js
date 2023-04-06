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
  let x, y, styleHeight; // the height in style like '100px', not a number

  const originalStyles = {};
  function backupStyle(name) {
    originalStyles[name] = textarea.style[name];
  }
  function restoreStyle(name) {
    if (name in originalStyles) {
      if (originalStyles[name] === undefined) {
        textarea.style.removeProperty(name);
      } else {
        textarea.style[name] = originalStyles[name];
      }
    }
  }

  function onUserResize(event) {
    if (isUserResized) return;
    if (x !== event.clientX || y !== event.clientY) {
      const newStyleHeight = textarea.style.height;
      if (styleHeight && styleHeight !== newStyleHeight) {
        isUserResized = true;
        textarea.style.removeProperty('max-height');
      }
      styleHeight = newStyleHeight;
    }

    x = event.clientX;
    y = event.clientY;
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
    try {
      if (isUserResized) return;
      if (textarea.offsetWidth <= 0 && textarea.offsetHeight <= 0) return;

      const {top, bottom} = overflowOffset();
      if (top < 0 || bottom < 0) return;

      const textareaStyle = getComputedStyle(textarea);
      const topBorderWidth = parseFloat(textareaStyle.borderTopWidth);
      const bottomBorderWidth = parseFloat(textareaStyle.borderBottomWidth);
      const isBorderBox = textareaStyle.boxSizing === 'border-box';
      const borderAddOn = isBorderBox ? topBorderWidth + bottomBorderWidth : 0;
      const maxHeight = parseFloat(textareaStyle.height) + bottom;
      const adjustedViewportMarginBottom = bottom < viewportMarginBottom ? bottom : viewportMarginBottom;

      textarea.style.maxHeight = `${maxHeight - adjustedViewportMarginBottom}px`;
      textarea.style.height = 'auto';
      textarea.style.height = `${textarea.scrollHeight + borderAddOn}px`;
      styleHeight = textarea.style.height;
    } finally {
      // ensure that the textarea is fully scrolled to the end
      // when the cursor is at the end during an input event
      if (textarea.selectionStart === textarea.selectionEnd &&
          textarea.selectionStart === textarea.value.length) {
        textarea.scrollTop = textarea.scrollHeight;
      }
    }
  }

  function onFormReset() {
    isUserResized = false;
    restoreStyle('height');
    restoreStyle('max-height');
  }

  textarea.addEventListener('mousemove', onUserResize);
  textarea.addEventListener('input', resizeToFit);
  textarea.form?.addEventListener('reset', onFormReset);
  backupStyle('height');
  backupStyle('max-height');
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
