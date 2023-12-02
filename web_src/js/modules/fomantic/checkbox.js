import $ from 'jquery';
import {generateAriaId} from './base.js';

const ariaPatchKey = '_giteaAriaPatchCheckbox';
const fomanticCheckboxFn = $.fn.checkbox;

// use our own `$.fn.checkbox` to patch Fomantic's checkbox module
export function initAriaCheckboxPatch() {
  if ($.fn.checkbox === ariaCheckboxFn) throw new Error('initAriaCheckboxPatch could only be called once');
  $.fn.checkbox = ariaCheckboxFn;
  ariaCheckboxFn.settings = fomanticCheckboxFn.settings;
}

// the patched `$.fn.checkbox` checkbox function
// * it does the one-time attaching on the first call
function ariaCheckboxFn(...args) {
  const ret = fomanticCheckboxFn.apply(this, args);
  for (const el of this) {
    if (el[ariaPatchKey]) continue;
    attachInit(el);
  }
  return ret;
}

function attachInit(el) {
  // Fomantic UI checkbox needs to be something like: <div class="ui checkbox"><label /><input /></div>
  // It doesn't work well with <label><input />...</label>
  // To make it work with aria, the "id"/"for" attributes are necessary, so add them automatically if missing.
  // In the future, refactor to use native checkbox directly, then this patch could be removed.
  el[ariaPatchKey] = {}; // record that this element has been patched
  const label = el.querySelector('label');
  const input = el.querySelector('input');
  if (!label || !input || input.getAttribute('id')) return;

  const id = generateAriaId();
  input.setAttribute('id', id);
  label.setAttribute('for', id);
}
