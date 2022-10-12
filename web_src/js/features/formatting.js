import {prettyNumber} from '../utils.js';

const {lang} = document.documentElement;
const dateFormatter = new Intl.DateTimeFormat(lang, {year: 'numeric', month: 'long', day: 'numeric'});

export function initFormattingReplacements() {
  // replace english formatted numbers with locale-specific separators
  for (const el of document.getElementsByClassName('js-pretty-number')) {
    const num = Number(el.getAttribute('data-value'));
    const formatted = prettyNumber(num, lang);
    if (formatted && formatted !== el.textContent) {
      el.textContent = formatted;
    }
  }

  // for each <time></time> tag, if it has the data-format="date" attribute, format
  // the text according to the user's chosen locale
  for (const timeElement of document.querySelectorAll('time[data-format="date"]')) {
    timeElement.textContent = dateFormatter.format(new Date(timeElement.dateTime));
  }
}
