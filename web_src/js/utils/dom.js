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

/* autosize a textarea to fit content */
/* Based on https://github.com/github/textarea-autosize */
export function autosize(textarea, {viewportMarginBottom = 100} = {}) {
  let isUserResized = false;
  let x, y, height;

  function onUserResize(event) {
    if (x !== event.clientX || y !== event.clientY) {
      const newHeight = textarea.style.height;
      if (height && height !== newHeight) {
        isUserResized = true;
        textarea.style.removeProperty('max-height');
        textarea.removeEventListener('mousemove', onUserResize);
      }
      height = newHeight;
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

    const top = offsetTop - document.defaultView.pageYOffset;
    const bottom = document.documentElement.clientHeight - (top + textarea.offsetHeight);
    return {top, bottom};
  }

  function sizeToFit() {
    try {
      if (isUserResized) return;
      if (textarea.offsetWidth <= 0 && textarea.offsetHeight <= 0) return;

      const {top, bottom} = overflowOffset();
      if (top < 0 || bottom < 0) return;

      const textareaStyle = getComputedStyle(textarea);
      const topBorderWidth = Number(textareaStyle.borderTopWidth.replace(/px/, ''));
      const bottomBorderWidth = Number(textareaStyle.borderBottomWidth.replace(/px/, ''));
      const isBorderBox = textareaStyle.boxSizing === 'border-box';
      const borderAddOn = isBorderBox ? topBorderWidth + bottomBorderWidth : 0;
      const maxHeight = Number(textareaStyle.height.replace(/px/, '')) + bottom;
      const adjustedViewportMarginBottom = bottom < viewportMarginBottom ? bottom : viewportMarginBottom;

      textarea.style.maxHeight = `${maxHeight - adjustedViewportMarginBottom}px`;
      textarea.style.height = 'auto';
      textarea.style.height = `${textarea.scrollHeight + borderAddOn}px`;
      height = textarea.style.height;
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
    textarea.style.removeProperty('height');
    textarea.style.removeProperty('max-height');
  }

  textarea.addEventListener('mousemove', onUserResize);
  textarea.addEventListener('keyup', sizeToFit);
  textarea.addEventListener('paste', sizeToFit);
  textarea.addEventListener('input', sizeToFit);
  const form = textarea.form;
  if (form) form.addEventListener('reset', onFormReset);
  if (textarea.value) sizeToFit();

  return {
    unsubscribe() {
      textarea.removeEventListener('mousemove', onUserResize);
      textarea.removeEventListener('keyup', sizeToFit);
      textarea.removeEventListener('input', sizeToFit);
      if (form) form.removeEventListener('reset', onFormReset);
    }
  };
}
