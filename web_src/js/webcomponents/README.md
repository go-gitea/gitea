# Web Components

This `webcomponents` directory contains the source code for the web components used in the Gitea Web UI.

https://developer.mozilla.org/en-US/docs/Web/Web_Components

# Guidelines

* These components are loaded in `<head>` (before DOM body),
  so they should have their own dependencies and should be very light,
  then they won't affect the page loading time too much.
* If the component is not a public one, it's suggested to have its own `Gitea` or `gitea-` prefix to avoid conflicts.
