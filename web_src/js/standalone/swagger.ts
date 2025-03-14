import SwaggerUI from 'swagger-ui-dist/swagger-ui-es-bundle.js';
import 'swagger-ui-dist/swagger-ui.css';

window.addEventListener('load', async () => {
  const url = document.querySelector('#swagger-ui').getAttribute('data-source');
  const res = await fetch(url);
  const spec = await res.json();

  // Make the page's protocol be at the top of the schemes list
  const proto = window.location.protocol.slice(0, -1);
  spec.schemes.sort((a: string, b: string) => {
    if (a === proto) return -1;
    if (b === proto) return 1;
    return 0;
  });

  SwaggerUI({
    spec,
    dom_id: '#swagger-ui',
    deepLinking: true,
    docExpansion: 'none',
    defaultModelRendering: 'model', // don't show examples by default, because they may be incomplete
    presets: [
      SwaggerUI.presets.apis,
    ],
    plugins: [
      SwaggerUI.plugins.DownloadUrl,
    ],
  });
});
