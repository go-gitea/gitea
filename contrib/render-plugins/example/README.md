# Example Frontend Render Plugin

This directory contains a minimal render plugin that highlights `.txt` files
with a custom color scheme. Use it as a starting point for your own plugins or
as a quick way to validate the dynamic plugin system locally.

## Files

- `manifest.json` &mdash; metadata (including the required `schemaVersion`) consumed by Gitea when installing a plugin
- `render.js` &mdash; an ES module that exports a `render(container, fileUrl)`
  function; it downloads the source file and renders it in a styled `<pre>`

## Build & Install

1. Create a zip archive that contains both files:

   ```bash
   cd contrib/render-plugins/example
   zip -r ../example-highlight-txt.zip manifest.json render.js
   ```

2. In the Gitea web UI, visit `Site Administration → Render Plugins`, upload
   `example-highlight-txt.zip`, and enable it.

3. Open any `.txt` file in a repository; the viewer will display the content in
   the custom colors to confirm the plugin is active.

Feel free to modify `render.js` to experiment with the API. The plugin runs in
the browser, so only standard Web APIs are available (no bundler is required
as long as the file stays a plain ES module).
