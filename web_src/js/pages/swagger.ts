import {load as loadYaml} from 'js-yaml';
import {isDarkTheme} from '../utils.ts';

function syncDarkModeClass(): void {
  document.documentElement.classList.toggle('dark-mode', isDarkTheme());
}

export async function initSwagger() {
  const elSwaggerUi = document.querySelector('#swagger-ui');
  if (!elSwaggerUi) return;

  // swagger-ui has built-in dark mode triggered by html.dark-mode class
  syncDarkModeClass();
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', syncDarkModeClass);

  const [{default: SwaggerUI}] = await Promise.all([
    import('swagger-ui-dist/swagger-ui-es-bundle.js'),
    import('swagger-ui-dist/swagger-ui.css'),
    import('../../css/swagger.css'),
  ]);

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
