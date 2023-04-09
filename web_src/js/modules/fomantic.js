import $ from 'jquery';
import {initAriaCheckboxPatch} from './aria/checkbox.js';
import {initAriaDropdownPatch} from './aria/dropdown.js';
import {svg} from '../svg.js';

export function initGiteaFomantic() {
  // Silence fomantic's error logging when tabs are used without a target content element
  $.fn.tab.settings.silent = true;
  // Disable the behavior of fomantic to toggle the checkbox when you press enter on a checkbox element.
  $.fn.checkbox.settings.enableEnterKey = false;

  // By default, use "exact match" for full text search
  $.fn.dropdown.settings.fullTextSearch = 'exact';
  // Do not use "cursor: pointer" for dropdown labels
  $.fn.dropdown.settings.className.label += ' gt-cursor-default';
  // Always use Gitea's SVG icons
  $.fn.dropdown.settings.templates.label = function(_value, text, preserveHTML, className) {
    const escape = $.fn.dropdown.settings.templates.escape;
    return escape(text, preserveHTML) + svg('octicon-x', 16, `${className.delete} icon`);
  };

  initFomanticApiPatch();

  // Use the patches to improve accessibility, these patches are designed to be as independent as possible, make it easy to modify or remove in the future.
  initAriaCheckboxPatch();
  initAriaDropdownPatch();
}

function initFomanticApiPatch() {
  //
  // Fomantic API module has some very buggy behaviors:
  //
  // If encodeParameters=true, it calls `urlEncodedValue` to encode the parameter.
  // However, `urlEncodedValue` just tries to "guess" whether the parameter is already encoded, by decoding the parameter and encoding it again.
  //
  // There are 2 problems:
  // 1. It may guess wrong, and skip encoding a parameter which looks like encoded.
  // 2. If the parameter can't be decoded, `decodeURIComponent` will throw an error, and the whole request will fail.
  //
  // This patch only fixes the second error behavior at the moment.
  //
  const patchKey = '_giteaFomanticApiPatch';
  const oldApi = $.api;
  $.api = $.fn.api = function(...args) {
    const apiCall = oldApi.bind(this);
    const ret = oldApi.apply(this, args);

    if (typeof args[0] !== 'string') {
      const internalGet = apiCall('internal', 'get');
      if (!internalGet.urlEncodedValue[patchKey]) {
        const oldUrlEncodedValue = internalGet.urlEncodedValue;
        internalGet.urlEncodedValue = function (value) {
          try {
            return oldUrlEncodedValue(value);
          } catch {
            // if Fomantic API module's `urlEncodedValue` throws an error, we encode it by ourselves.
            return encodeURIComponent(value);
          }
        };
        internalGet.urlEncodedValue[patchKey] = true;
      }
    }
    return ret;
  };
  $.api.settings = oldApi.settings;
}
