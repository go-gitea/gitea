# Web Components

This `webcomponents` directory contains the source code for the web components used in the Gitea Web UI.

https://developer.mozilla.org/en-US/docs/Web/Web_Components

# Guidelines

* These components are loaded in `<head>` (before DOM body) in a separate entry point, they need to be lightweight to not affect the page loading time too much.
* Do not import `svg.js` into a web component because that file is currently not tree-shakeable, import svg files individually insteat.
* All our components must be added to `webpack.config.js` so they work correctly in Vue.
