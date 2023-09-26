import $ from 'jquery';
import {initAriaCheckboxPatch} from './aria/checkbox.js';
import {initAriaDropdownPatch} from './aria/dropdown.js';
import {initAriaModalPatch} from './aria/modal.js';
import {svg} from '../svg.js';

export const fomanticMobileScreen = window.matchMedia('only screen and (max-width: 767.98px)');

export function initGiteaFomantic() {
  // Silence fomantic's error logging when tabs are used without a target content element
  $.fn.tab.settings.silent = true;
  // Disable the behavior of fomantic to toggle the checkbox when you press enter on a checkbox element.
  $.fn.checkbox.settings.enableEnterKey = false;

  // By default, use "exact match" for full text search
  $.fn.dropdown.settings.fullTextSearch = 'exact';
  // Do not use "cursor: pointer" for dropdown labels
  $.fn.dropdown.settings.className.label += ' gt-cursor-default';
  // The default selector has a bug: if there is a "search input" in the "menu", Fomantic will only "focus the input" but not "toggle the menu" when the "dropdown icon" is clicked.
  // Actually, the "search input in menu" shouldn't be considered as the dropdown's input
  $.fn.dropdown.settings.selector.search = '> input.search, :not(.menu) > .search > input, :not(.menu) input.search';
  // Always use Gitea's SVG icons
  $.fn.dropdown.settings.templates.label = function(_value, text, preserveHTML, className) {
    const escape = $.fn.dropdown.settings.templates.escape;
    return escape(text, preserveHTML) + svg('octicon-x', 16, `${className.delete} icon`);
  };

  // stand-in for removed transition module
  $.fn.transition = function (arg) {
    if (arg === 'is supported') return true;
    if (arg === 'is animating') return false;
    if (arg === 'is inward') return false;
    if (arg === 'is outward') return false;
    if (arg === 'stop all') return;

    const isIn = arg?.animation?.endsWith(' in');
    const isOut = arg?.animation?.endsWith(' out');
    const isScale = arg?.animation?.includes('scale');

    let ret;
    if (arg === 'show' || isIn) {
      arg?.onStart?.(this);
      ret = this.each((_, el) => {
        el.classList.remove('hidden');
        el.classList.add('visible');
        if (isIn) el.classList.add('transition');
        if (arg?.displayType) el.style.setProperty('display', arg.displayType, 'important');
        arg?.onShow?.(this);
      });
      arg?.onComplete?.(this);
    } else if (arg === 'hide' || isOut) {
      arg?.onStart?.(this);
      ret = this.each((_, el) => {
        el.classList.add('hidden');
        el.classList.remove('visible');
        // don't remove the transition class because fomantic didn't do it either
        el.style.removeProperty('display');
        arg?.onHidden?.(this);
      });
      arg?.onComplete?.(this);
    } else if (isScale) {
      arg?.onStart?.(this);
      ret = this.each((_, el) => {
        if (el.classList.contains('hidden')) {
          el.classList.remove('hidden');
          el.classList.add('visible');
          arg?.onShow?.(this);
        } else if (el.classList.contains('visible')) {
          el.classList.remove('visible');
          el.classList.add('hidden');
          el.style.removeProperty('display');
          arg?.onHidden?.(this);
        }
      });
      arg.onComplete?.(this);
    }
    return ret;
  };

  initFomanticApiPatch();

  // Use the patches to improve accessibility, these patches are designed to be as independent as possible, make it easy to modify or remove in the future.
  initAriaCheckboxPatch();
  initAriaDropdownPatch();
  initAriaModalPatch();
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
