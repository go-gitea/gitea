// AVOID importing other unneeded main site JS modules to prevent unnecessary code and dependencies and chunks.
//
// Swagger JS is standalone because it is also used by external render like "File View -> OpenAPI render",
// and it doesn't need any code from main site's modules (at the moment).
//
// In the future, if there are common utilities needed by both main site and standalone Swagger,
// we can merge this standalone module into "index.ts", do pay attention to the following problems:
// * HINT: SWAGGER-OPENAPI-VIEWER: there are different places rendering the swagger UI.
// * Handle CSS styles carefully for different cases (standalone page, embedded in iframe)
// * Take care of the JS code introduced by "index.ts" and "iife.ts", there might be global variable dependency and event listeners.

import '../css/swagger.css';
import SwaggerUI from 'swagger-ui-dist/swagger-ui-es-bundle.js';
import 'swagger-ui-dist/swagger-ui.css';
import {load as loadYaml} from 'js-yaml';

function syncDarkModeClass(): void {
  // if the viewer is embedded in an iframe (external render), use the parent's theme (passed via query param)
  // otherwise, if it is for Gitea's API, it is a standalone page, use the site's theme (detected from theme CSS variable)
  const url = new URL(window.location.href);
  const giteaIsDarkTheme = url.searchParams.get('gitea-is-dark-theme') ??
    window.getComputedStyle(document.documentElement).getPropertyValue('--is-dark-theme').trim();
  const isDark = giteaIsDarkTheme ? giteaIsDarkTheme === 'true' : window.matchMedia('(prefers-color-scheme: dark)').matches;
  document.documentElement.classList.toggle('dark-mode', isDark);
}

async function initSwaggerUI() {
  // swagger-ui has built-in dark mode triggered by html.dark-mode class
  syncDarkModeClass();
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', syncDarkModeClass);

  const elSwaggerUi = document.querySelector('#swagger-ui')!;
  const url = elSwaggerUi.getAttribute('data-source')!;
  let spec: any;
  if (url) {
    const res = await fetch(url); // eslint-disable-line no-restricted-globals
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
}

initSwaggerUI();
