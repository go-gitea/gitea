import SwaggerUI from 'swagger-ui-dist/swagger-ui-es-bundle.js';
import 'swagger-ui-dist/swagger-ui.css';
import {load as loadYaml} from 'js-yaml';
import {GET} from '../modules/fetch.ts';

window.addEventListener('load', async () => {
  const elSwaggerUi = document.querySelector('#swagger-ui')!;
  const url = elSwaggerUi.getAttribute('data-source')!;
  let spec: any;
  if (url) {
    const res = await GET(url);
    spec = await res.json();
  } else {
    const elSpecContent = elSwaggerUi.querySelector<HTMLTextAreaElement>('.swagger-spec-content')!;
    const filename = elSpecContent.getAttribute('data-spec-filename');
    const isJson = filename?.toLowerCase().endsWith('.json');
    spec = isJson ? JSON.parse(elSpecContent.value) : loadYaml(elSpecContent.value);
  }

  // Make the page's protocol be at the top of the schemes list
  const proto = window.location.protocol.slice(0, -1);
  if (spec?.schemes) {
    spec.schemes.sort((a: string, b: string) => {
      if (a === proto) return -1;
      if (b === proto) return 1;
      return 0;
    });
  }

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
