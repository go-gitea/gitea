import {isDarkTheme, parseDom} from '../utils.ts';
import {makeCodeCopyButton} from './codecopy.ts';
import {displayError} from './common.ts';
import {createElementFromAttrs, queryElems} from '../utils/dom.ts';
import {html, htmlRaw} from '../utils/html.ts';
import {load as loadYaml} from 'js-yaml';
import type {MermaidConfig} from 'mermaid';

const {mermaidMaxSourceCharacters} = window.config;

const iframeCss = `:root {color-scheme: normal}
body {margin: 0; padding: 0; overflow: hidden}
#mermaid {display: block; margin: 0 auto}`;

function isSourceTooLarge(source: string) {
  return mermaidMaxSourceCharacters >= 0 && source.length > mermaidMaxSourceCharacters;
}

function parseYamlInitConfig(source: string): MermaidConfig | null {
  // ref: https://github.com/mermaid-js/mermaid/blob/develop/packages/mermaid/src/diagram-api/regexes.ts
  const yamlFrontMatterRegex = /^---\s*[\n\r](.*?)[\n\r]---\s*[\n\r]+/s;
  const frontmatter = (yamlFrontMatterRegex.exec(source) || [])[1];
  if (!frontmatter) return null;
  try {
    return (loadYaml(frontmatter) as {config: MermaidConfig})?.config;
  } catch {
    console.error('invalid or unsupported mermaid init YAML config', frontmatter);
  }
  return null;
}

function parseJsonInitConfig(source: string): MermaidConfig | null {
  // https://mermaid.js.org/config/directives.html#declaring-directives
  // Do as dirty as mermaid does: https://github.com/mermaid-js/mermaid/blob/develop/packages/mermaid/src/utils.ts
  // It can even accept invalid JSON string like:
  // %%{initialize: { 'logLevel': 'fatal', "theme":'dark', 'startOnLoad': true } }%%
  const jsonInitConfigRegex = /%%\{\s*(init|initialize)\s*:\s*(.*?)\}%%/s;
  const jsonInitText = (jsonInitConfigRegex.exec(source) || [])[2];
  if (!jsonInitText) return null;
  try {
    const processed = jsonInitText.trim().replace(/'/g, '"');
    return JSON.parse(processed);
  } catch {
    console.error('invalid or unsupported mermaid init JSON config', jsonInitText);
  }
  return null;
}

function configValueIsElk(layoutOrRenderer: string | undefined) {
  if (typeof layoutOrRenderer !== 'string') return false;
  return layoutOrRenderer === 'elk' || layoutOrRenderer.startsWith('elk.');
}

function configContainsElk(config: MermaidConfig | null) {
  if (!config) return false;
  // Check the layout from the following properties:
  // * config.layout
  // * config.{any-diagram-config}.defaultRenderer
  //   Although only a few diagram types like "flowchart" support "defaultRenderer",
  //   as long as there is no side effect, here do a general check for all properties of "config", for ease of maintenance
  return configValueIsElk(config.layout) || Object.values(config).some((diagCfg) => configValueIsElk(diagCfg?.defaultRenderer));
}

export function sourceNeedsElk(source: string) {
  if (isSourceTooLarge(source)) return false;
  const configYaml = parseYamlInitConfig(source), configJson = parseJsonInitConfig(source);
  return configContainsElk(configYaml) || configContainsElk(configJson);
}

async function loadMermaid(needElkRender: boolean) {
  const mermaidPromise = import(/* webpackChunkName: "mermaid" */'mermaid');
  const elkPromise = needElkRender ? import(/* webpackChunkName: "mermaid-layout-elk" */'@mermaid-js/layout-elk') : null;
  const results = await Promise.all([mermaidPromise, elkPromise]);
  return {
    mermaid: results[0].default,
    elkLayouts: results[1]?.default,
  };
}

let elkLayoutsRegistered = false;

export async function initMarkupCodeMermaid(elMarkup: HTMLElement): Promise<void> {
  // .markup code.language-mermaid
  const mermaidBlocks: Array<{source: string, parentContainer: HTMLElement}> = [];
  const attrMermaidRendered = 'data-markup-mermaid-rendered';
  let needElkRender = false;
  for (const elCodeBlock of queryElems(elMarkup, 'code.language-mermaid')) {
    const parentContainer = elCodeBlock.closest('pre')!; // it must exist, if no, there must be a bug
    if (parentContainer.hasAttribute(attrMermaidRendered)) continue;
    parentContainer.setAttribute(attrMermaidRendered, 'true');

    const source = elCodeBlock.textContent ?? '';
    needElkRender = needElkRender || sourceNeedsElk(source);
    mermaidBlocks.push({source, parentContainer});
  }
  if (!mermaidBlocks.length) return;

  const {mermaid, elkLayouts} = await loadMermaid(needElkRender);
  if (elkLayouts && !elkLayoutsRegistered) {
    mermaid.registerLayoutLoaders(elkLayouts);
    elkLayoutsRegistered = true;
  }
  mermaid.initialize({
    startOnLoad: false,
    theme: isDarkTheme() ? 'dark' : 'neutral', // TODO: maybe it should use "darkMode" to adopt more user-specified theme instead of just "dark" or "neutral"
    securityLevel: 'strict',
    suppressErrorRendering: true,
  });

  // mermaid is a globally shared instance, its document also says "Multiple calls to this function will be enqueued to run serially."
  // so here we just simply render the mermaid blocks one by one, no need to do "Promise.all" concurrently
  for (const block of mermaidBlocks) {
    const {source, parentContainer} = block;
    if (isSourceTooLarge(source)) {
      displayError(parentContainer, new Error(`Mermaid source of ${source.length} characters exceeds the maximum allowed length of ${mermaidMaxSourceCharacters}.`));
      continue;
    }

    try {
      // render the mermaid diagram to svg text, and parse it to a DOM node
      const {svg: svgText, bindFunctions} = await mermaid.render('mermaid', source, parentContainer);
      const svgDoc = parseDom(svgText, 'image/svg+xml');
      const svgNode = (svgDoc.documentElement as unknown) as SVGSVGElement;

      // create an iframe to sandbox the svg with styles, and set correct height by reading svg's viewBox height
      const iframe = document.createElement('iframe');
      iframe.classList.add('markup-content-iframe', 'is-loading');
      iframe.srcdoc = html`<html><head><style>${htmlRaw(iframeCss)}</style></head><body></body></html>`;

      // although the "viewBox" is optional, mermaid's output should always have a correct viewBox with width and height
      const iframeHeightFromViewBox = Math.ceil(svgNode.viewBox?.baseVal?.height ?? 0);
      if (iframeHeightFromViewBox) iframe.style.height = `${iframeHeightFromViewBox}px`;

      // the iframe will be fully reloaded if its DOM context is changed (e.g.: moved in the DOM tree).
      // to avoid unnecessary reloading, we should insert the iframe to its final position only once.
      iframe.addEventListener('load', () => {
        // same origin, so we can operate "iframe body" and all elements directly
        const iframeBody = iframe.contentDocument!.body;
        iframeBody.append(svgNode);
        bindFunctions?.(iframeBody); // follow "mermaid.render" doc, attach event handlers to the svg's container

        // according to mermaid, the viewBox height should always exist, here just a fallback for unknown cases.
        // and keep in mind: clientHeight can be 0 if the element is hidden (display: none).
        if (!iframeHeightFromViewBox && iframeBody.clientHeight) iframe.style.height = `${iframeBody.clientHeight}px`;
        iframe.classList.remove('is-loading');
      });

      const container = createElementFromAttrs('div', {class: 'mermaid-block'}, iframe, makeCodeCopyButton({'data-clipboard-text': source}));
      parentContainer.replaceWith(container);
    } catch (err) {
      displayError(parentContainer, err);
    }
  }
}
