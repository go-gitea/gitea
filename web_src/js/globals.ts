import jquery from 'jquery'; // eslint-disable-line no-restricted-imports
import htmx from 'htmx.org'; // eslint-disable-line no-restricted-imports

// Some users still use inline scripts and expect jQuery to be available globally.
// To avoid breaking existing users and custom plugins, import jQuery globally without ES module.
window.$ = window.jQuery = jquery;

// There is a bug in htmx, it incorrectly checks "readyState === 'complete'" when the DOM tree is ready and won't trigger DOMContentLoaded
// The bug makes htmx impossible to be loaded from an ES module: importing the htmx in onDomReady will make htmx skip its initialization.
// ref: https://github.com/bigskysoftware/htmx/pull/3365
window.htmx = htmx;

// https://htmx.org/reference/#config
htmx.config.requestClass = 'is-loading';
htmx.config.scrollIntoViewOnBoost = false;
