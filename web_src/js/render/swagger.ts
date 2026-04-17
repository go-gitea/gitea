// AVOID importing other unneeded main site JS modules to prevent unnecessary code and dependencies and chunks.
// This module is used by both the Gitea API page and the frontend external render.
// It doesn't need any code from main site's modules (at the moment).

import SwaggerUI from 'swagger-ui-dist/swagger-ui-es-bundle.js';
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

export async function initSwaggerUI(container: HTMLElement, opts: {specText: string}): Promise<void> {
  // swagger-ui has built-in dark mode triggered by html.dark-mode class
  syncDarkModeClass();
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', syncDarkModeClass);

  let spec: any;
  const specText = opts.specText.trim();
  if (specText.startsWith('{')) {
    spec = JSON.parse(specText);
  } else {
    spec = loadYaml(specText);
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
    domNode: container,
    deepLinking: window.location.protocol !== 'about:', // pushState fails inside about:srcdoc iframes
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
