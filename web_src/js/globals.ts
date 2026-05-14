import jquery from 'jquery'; // eslint-disable-line no-restricted-imports

// Some users still use inline scripts and expect jQuery to be available globally.
// To avoid breaking existing users and custom plugins, import jQuery globally without ES module.
window.$ = window.jQuery = jquery;
