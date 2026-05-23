// INVARIANT: the source CSS file for this entry must be named "swagger.css".
// The prod-time manifest parser (modules/public/manifest.go:parseManifest) synthesises the
// lookup key as "<dir>/<entry-name><ext>" (entry name == "swagger"), so it maps to
// "css/swagger.css" regardless of the real filename. The dev-mode resolver
// (modules/public/vitedev.go:detectWebSrcPath) stat-checks the literal path
// "web_src/css/swagger.css". Both must agree for {{AssetURI "css/swagger.css"}} in
// templates/swagger/openapi-viewer.tmpl to work in all run modes.
import '../css/swagger.css';
import {initSwaggerUI} from './render/swagger.ts';

async function initGiteaAPIViewer() {
  const elSwaggerUi = document.querySelector<HTMLElement>('#swagger-ui')!;
  const url = elSwaggerUi.getAttribute('data-source')!;
  const res = await fetch(url); // eslint-disable-line no-restricted-globals
  // HINT: SWAGGER-CSS-IMPORT: this is used in the standalone page which already has the related CSS imported by `<link>`
  await initSwaggerUI(elSwaggerUi, {specText: await res.text()});
}

initGiteaAPIViewer();
