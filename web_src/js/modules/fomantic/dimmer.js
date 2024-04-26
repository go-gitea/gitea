import $ from 'jquery';
import {queryElemChildren} from '../../utils/dom.js';

export function initFomanticDimmer() {
  // stand-in for removed dimmer module
  $.fn.dimmer = function (arg0, $el) {
    if (arg0 === 'add content') {
      const existingDimmer = document.querySelector('body > .ui.dimmer');
      if (existingDimmer) {
        queryElemChildren(existingDimmer, '*', (el) => el.remove());
        this._dimmer = existingDimmer;
      } else {
        this._dimmer = document.createElement('div');
        this._dimmer.classList.add('ui', 'dimmer');
        document.body.append(this._dimmer);
      }
      this._dimmer.append($el[0]);
    } else if (arg0 === 'get dimmer') {
      return $(this._dimmer);
    } else if (arg0 === 'show') {
      this._dimmer.classList.add('active');
      document.body.classList.add('tw-overflow-hidden');
    } else if (arg0 === 'hide') {
      this._dimmer.classList.remove('active');
      document.body.classList.remove('tw-overflow-hidden');
    }
    return this;
  };
}
