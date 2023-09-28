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

  const transitionNopBehaviors = new Set([
    'clear queue', 'stop', 'stop all', 'destroy',
    'force repaint', 'repaint', 'reset',
    'looping', 'remove looping', 'disable', 'enable',
    'set duration', 'save conditions', 'restore conditions',
  ]);
  // stand-in for removed transition module
  $.fn.transition = function (arg0, arg1, arg2) {
    if (arg0 === 'is supported') return true;
    if (arg0 === 'is animating') return false;
    if (arg0 === 'is inward') return false;
    if (arg0 === 'is outward') return false;

    let argObj;
    if (typeof arg0 === 'string') {
      // many behaviors are no-op now. https://fomantic-ui.com/modules/transition.html#/usage
      if (transitionNopBehaviors.has(arg0)) return this;
      // now, the arg0 is an animation name, the syntax: (animation, duration, complete)
      argObj = {animation: arg0, ...(arg1 && {duration: arg1}), ...(arg2 && {onComplete: arg2})};
    } else if (typeof arg0 === 'object') {
      argObj = arg0;
    } else {
      throw new Error(`invalid argument: ${arg0}`);
    }

    const isAnimationIn = argObj.animation?.startsWith('show') || argObj.animation?.endsWith(' in');
    const isAnimationOut = argObj.animation?.startsWith('hide') || argObj.animation?.endsWith(' out');
    this.each((_, el) => {
      let toShow = isAnimationIn;
      if (!isAnimationIn && !isAnimationOut) {
        // If the animation is not in/out, then it must be a toggle animation.
        // Fomantic uses computed styles to check "visibility", but to avoid unnecessary arguments, here it only checks the class.
        toShow = this.hasClass('hidden'); // maybe it could also check "!this.hasClass('visible')", leave it to the future until there is a real problem.
      }
      argObj.onStart?.call(el);
      if (toShow) {
        el.classList.remove('hidden');
        el.classList.add('visible', 'transition');
        if (argObj.displayType) el.style.setProperty('display', argObj.displayType, 'important');
        argObj.onShow?.call(el);
      } else {
        el.classList.add('hidden');
        el.classList.remove('visible'); // don't remove the transition class because the Fomantic animation style is `.hidden.transition`.
        el.style.removeProperty('display');
        argObj.onHidden?.call(el);
      }
      argObj.onComplete?.call(el);
    });
    return this;
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
