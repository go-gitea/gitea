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
