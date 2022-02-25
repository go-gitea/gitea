import {prettyNumber} from '../utils.js';

const {lang} = document.documentElement;

export function initFormattingReplacements() {
  // replace english formatted numbers with locale-specific separators
  for (const el of document.getElementsByClassName('js-pretty-number')) {
    const num = Number(el.getAttribute('data-value'));
    const formatted = prettyNumber(num, lang);
    if (formatted && formatted !== el.textContent) {
      el.textContent = formatted;
    }
  }
}
