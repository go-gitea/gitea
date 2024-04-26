import $ from 'jquery';
import {showElem, hideElem, queryElemChildren} from '../../utils/dom.js';

export function initFomanticDimmer() {
  // stand-in for removed dimmer module
  $.fn.dimmer = function (arg0, $el) {
    if (arg0 === 'add content') {
      this._dimmer = document.createElement('div');
      queryElemChildren(document.body, '.ui.dimmer', (el) => el.remove());
      this._dimmer.classList.add('ui', 'dimmer', 'tw-hidden');
      this._dimmer.append($el[0]);
      document.body.append(this._dimmer);
    } else if (arg0 === 'get dimmer') {
      return $(this._dimmer);
    } else if (arg0 === 'show') {
      this._dimmer.classList.add('active');
      showElem(this._dimmer);
      document.body.classList.add('tw-overflow-hidden');
    } else if (arg0 === 'hide') {
      this._dimmer.classList.remove('active');
      hideElem(this._dimmer);
      document.body.classList.remove('tw-overflow-hidden');
    } else {
      return this;
    }
  };
}
