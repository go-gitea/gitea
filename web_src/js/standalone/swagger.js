import SwaggerUI from 'swagger-ui-dist/swagger-ui-es-bundle.js';
import 'swagger-ui-dist/swagger-ui.css';
import {parseUrl} from '../utils.js';
import {load} from 'js-yaml';

// This code is shared for our own spec as well as user-defined specs via files in repo
window.addEventListener('load', async () => {
  const url = parseUrl(document.getElementById('swagger-ui').getAttribute('data-source'));
  const res = await fetch(url);
  const text = await res.text();
  const spec = /\.ya?ml$/i.test(url.pathname) ? load(text) : JSON.parse(text);
  const isOwnSpec = url.pathname.endsWith('/swagger.v1.json');

  if (isOwnSpec) {
    // Make the page's protocol be at the top of the schemes list
    const proto = window.location.protocol.slice(0, -1);
    spec.schemes.sort((a, b) => {
      if (a === proto) return -1;
      if (b === proto) return 1;
      return 0;
    });
  }

  const ui = SwaggerUI({
    spec,
    dom_id: '#swagger-ui',
    deepLinking: true,
    docExpansion: 'none',
    presets: [
      SwaggerUI.presets.apis,
    ],
    plugins: [
      SwaggerUI.plugins.DownloadUrl,
    ],
  });

  window.ui = ui;
});
