import jquery from 'jquery'; // eslint-disable-line no-restricted-imports -- this is where the global $ comes from

// Some users still use inline scripts and expect jQuery to be available globally.
// To avoid breaking existing users and custom plugins, import jQuery globally without ES module.
Object.assign(window, {$: jquery, jQuery: jquery});
