import {prettyNumber} from '../utils.js';

const {lang} = document.documentElement;

const dateFormatter = new Intl.DateTimeFormat(lang, {year: 'numeric', month: 'long', day: 'numeric'});
const shortDateFormatter = new Intl.DateTimeFormat(lang, {year: 'numeric', month: 'short', day: 'numeric'});
const dateTimeFormatter = new Intl.DateTimeFormat(lang, {year: 'numeric', month: 'short', day: 'numeric', hour: 'numeric', minute: 'numeric', second: 'numeric'});

export function initFormattingReplacements() {
  // replace english formatted numbers with locale-specific separators
  for (const el of document.getElementsByClassName('js-pretty-number')) {
    const num = Number(el.getAttribute('data-value'));
    const formatted = prettyNumber(num, lang);
    if (formatted && formatted !== el.textContent) {
      el.textContent = formatted;
    }
  }

  // for each <time></time> tag, if it has the data-forma attribute, format
  // the text according to the user's chosen locale and formatter
  formatAllTimeElements();
}

function formatAllTimeElements() {
  const formats = ['date', 'short-date', 'date-time'];
  for (const f of formats) {
    formatTimeElements(f);
  }
}

function formatTimeElements(format) {
  let formatter;
  switch (format) {
    case 'date':
      formatter = dateFormatter;
      break;
    case 'short-date':
      formatter = shortDateFormatter;
      break;
    case 'date-time':
      formatter = dateTimeFormatter;
      break;
    default:
      throw new Error('Unknown format');
  }
  for (const timeElement of document.querySelectorAll(`time[data-format="${format}"]`)) {
    timeElement.textContent = formatter.format(new Date(timeElement.dateTime));
  }
}
