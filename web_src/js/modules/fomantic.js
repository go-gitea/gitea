import $ from 'jquery';
import {initAriaCheckboxPatch} from './aria/checkbox.js';
import {initAriaDropdownPatch} from './aria/dropdown.js';
import {svg} from '../svg.js';

export function initGiteaFomantic() {
  // Silence fomantic's error logging when tabs are used without a target content element
  $.fn.tab.settings.silent = true;
  // Disable the behavior of fomantic to toggle the checkbox when you press enter on a checkbox element.
  $.fn.checkbox.settings.enableEnterKey = false;

  // Prevent Fomantic from guessing the popup direction.
  // Otherwise, if the viewport height is small, Fomantic would show the popup upward,
  // if the dropdown is at the beginning of the page, then the top part would be clipped by the window view, eg: Issue List "Sort" dropdown
  $.fn.dropdown.settings.direction = 'downward';
  // By default, use "exact match" for full text search
  $.fn.dropdown.settings.fullTextSearch = 'exact';
  // Do not use "cursor: pointer" for dropdown labels
  $.fn.dropdown.settings.className.label += ' gt-cursor-default';
  // Always use Gitea's SVG icons
  $.fn.dropdown.settings.templates.label = function(_value, text, preserveHTML, className) {
    const escape = $.fn.dropdown.settings.templates.escape;
    return escape(text, preserveHTML) + svg('octicon-x', 16, `${className.delete} icon`);
  };

  // Use the patches to improve accessibility, these patches are designed to be as independent as possible, make it easy to modify or remove in the future.
  initAriaCheckboxPatch();
  initAriaDropdownPatch();
}
