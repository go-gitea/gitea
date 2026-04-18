// FIXME: INCORRECT-VITE-MANIFEST-PARSER: it just happens to work for current dependencies
// If this module depends on another one and that one imports "swagger.css", then {{AssetURI "css/swagger.css"}} won't work
import '../css/swagger-standalone.css';
import {initSwaggerUI} from './render/swagger.ts';

async function initGiteaAPIViewer() {
  const elSwaggerUi = document.querySelector<HTMLElement>('#swagger-ui')!;
  const url = elSwaggerUi.getAttribute('data-source')!;
  const res = await fetch(url); // eslint-disable-line no-restricted-globals
  // HINT: SWAGGER-CSS-IMPORT: this is used in the standalone page which already has the related CSS imported by `<link>`
  await initSwaggerUI(elSwaggerUi, {specText: await res.text()});
}

initGiteaAPIViewer();
