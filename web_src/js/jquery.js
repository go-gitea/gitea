import $ from 'jquery';

window.$ = window.jQuery = $;

function getComputedStyleProperty(el, prop) {
  const cs = el ? window.getComputedStyle(el) : null;
  return cs ? cs[prop] : null;
}

const defaultDisplayMap = {};
function getDefaultDisplay(el) {
  let display = defaultDisplayMap[el.nodeName];
  if (display) return display;

  const temp = el.ownerDocument.body.appendChild(el.ownerDocument.createElement(el.nodeName));
  display = getComputedStyleProperty(el, 'display');
  temp.parentNode.removeChild(temp);

  display = display === 'none' ? 'block' : display;
  defaultDisplayMap[el.nodeName] = display;
  return display;
}

function showHide(elements, show) {
  for (const el of elements) {
    if (!el || !el.classList) continue;
    if (show) {
      // at the moment, there are various hiding-methods in Gitea
      // in the future, after they are all refactored by "gt-hidden" class, we can remove all others
      el.removeAttribute('hidden');
      el.classList.remove('hide', 'gt-hidden');
      if (el.style.display === 'none') el.style.removeProperty('display');

      if (getComputedStyleProperty(el, 'display') === 'none') {
        // after removing all "hidden" related classes/attributes, if the element is still hidden,
        // maybe it already has another class with "display: none", so we need to set the "display: xxx" to its style
        el.style.display = getDefaultDisplay(el);
      }
    } else {
      el.classList.add('gt-hidden');
    }
  }
  return elements;
}

function warnDeprecated(fn) {
  if (!window.config?.runModeIsProd) {
    console.warn(`jQuery.${fn}() is deprecated, add/remove Gitea specialized helper "gt-hidden" class instead`);
  }
}

window.jQuery.fn.extend({
  show () {
    warnDeprecated('show');
    return showHide(this, true);
  },
  hide () {
    warnDeprecated('hide');
    return showHide(this, false);
  },
  toggle (state) {
    warnDeprecated('toggle');
    if (typeof state === 'boolean') {
      return showHide(this, state);
    }
    return this.each(function () {
      if (getComputedStyleProperty(this, 'display') === 'none') {
        $(this).show();
      } else {
        $(this).hide();
      }
    });
  }
});
