# Go + WASM Render Plugin Example

This example shows how to build a frontend render plugin whose heavy lifting
runs inside a WebAssembly module compiled from Go. The plugin loads the WASM
binary in the browser, asks Go to post-process the fetched file content, and
then renders the result inside the file viewer.

## Files

- `manifest.json` &mdash; plugin metadata consumed by the Gitea backend
- `render.js` &mdash; the ES module entry point that initializes the Go runtime and
  renders files handled by the plugin
- `wasm/` &mdash; contains the Go source that compiles to `plugin.wasm`
- `wasm_exec.js` &mdash; the Go runtime shim required by all Go-generated WASM
  binaries (copied verbatim from the Go distribution)
- `build.sh` &mdash; helper script that builds `plugin.wasm` and produces a zip
  archive ready for upload

As with other plugins, declare any Gitea endpoints or external hosts the WASM
module needs to call inside the `permissions` array in `manifest.json`. Without
an explicit entry, the plugin may only download the file that is currently being
rendered.

## Build & Install

1. Build the WASM binary and zip archive:

   ```bash
   cd contrib/render-plugins/example-wasm
   ./build.sh
   ```

   The script requires Go 1.21+ on your PATH. It stores the compiled WASM and
   an installable `example-go-wasm.zip` under `dist/`.

2. In the Gitea web UI, visit `Site Administration → Render Plugins`, upload
   `dist/example-go-wasm.zip`, and enable the plugin.

3. Open any file whose name ends with `.wasmnote`; the viewer will show the
   processed output from the Go code running inside WebAssembly.

## How It Works

- `wasm/main.go` exposes a single `wasmProcessFile` function to JavaScript. It
  uppercases each line, prefixes it with the line number, and runs entirely
  inside WebAssembly compiled from Go.
- `render.js` injects the Go runtime (`wasm_exec.js`), instantiates the compiled
  module, and caches the exported `wasmProcessFile` function.
- During initialization the frontend passes the sniffed MIME type and the first
  1 KiB of file data to the plugin (`options.mimeType`/`options.headChunk`),
  allowing renderers to make decisions without issuing extra network requests.
- During rendering the plugin downloads the target file, passes the contents to
  Go, and displays the transformed text with minimal styling.

Feel free to modify the Go source or the JS wrapper to experiment with richer
interfaces between JavaScript and WebAssembly.
