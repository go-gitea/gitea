import jquery from 'jquery'; // eslint-disable-line no-restricted-imports
import htmx from 'htmx.org'; // eslint-disable-line no-restricted-imports
import 'idiomorph/htmx'; // eslint-disable-line no-restricted-imports

window.$ = window.jQuery = jquery;
window.htmx = htmx;

// https://htmx.org/reference/#config
htmx.config.requestClass = 'is-loading';
htmx.config.scrollIntoViewOnBoost = false;
