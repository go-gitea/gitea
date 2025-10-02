import {isDarkTheme} from '../utils.ts';
import {makeCodeCopyButton} from './codecopy.ts';
import {displayError} from './common.ts';
import {queryElems} from '../utils/dom.ts';
import {html, htmlRaw} from '../utils/html.ts';

const {excalidrawMaxSourceCharacters} = window.config;

const iframeCss = `body { margin: 0; } svg { max-width: 100%; height: auto; }`;

export async function initMarkupCodeExcalidraw(elMarkup: HTMLElement): Promise<void> {
  queryElems(elMarkup, 'code.language-excalidraw', async (el) => {
    const {exportToSvg} = await import(/* webpackChunkName: "excalidraw/utils" */ '@excalidraw/utils');

    const pre = el.closest('pre');
    if (pre.hasAttribute('data-render-done')) return;

    const source = el.textContent;
    if (excalidrawMaxSourceCharacters >= 0 && source.length > excalidrawMaxSourceCharacters) {
      displayError(pre, new Error(`Excalidraw source of ${source.length} characters exceeds the maximum allowed length of ${excalidrawMaxSourceCharacters}.`));
      return;
    }

    let excalidrawJson;
    try {
      excalidrawJson = JSON.parse(source);
    } catch (err) {
      displayError(pre, new Error(`Invalid Excalidraw JSON: ${err}`));
      return;
    }

    try {
      const svg = await exportToSvg({
        elements: excalidrawJson.elements,
        appState: {
          ...excalidrawJson.appState,
          exportWithDarkMode: isDarkTheme(),
        },
        files: excalidrawJson.files,
        skipInliningFonts: true,
      });
      const iframe = document.createElement('iframe');
      iframe.classList.add('markup-content-iframe', 'tw-invisible');
      iframe.srcdoc = html`<html><head><style>${htmlRaw(iframeCss)}</style></head><body>${htmlRaw(svg.outerHTML)}</body></html>`;

      const excalidrawBlock = document.createElement('div');
      excalidrawBlock.classList.add('excalidraw-block', 'is-loading', 'tw-hidden');
      excalidrawBlock.append(iframe);

      const btn = makeCodeCopyButton();
      btn.setAttribute('data-clipboard-text', source);
      excalidrawBlock.append(btn);

      const updateIframeHeight = () => {
        const body = iframe.contentWindow?.document?.body;
        if (body) {
          iframe.style.height = `${body.clientHeight}px`;
        }
      };
      iframe.addEventListener('load', () => {
        pre.replaceWith(excalidrawBlock);
        excalidrawBlock.classList.remove('tw-hidden');
        updateIframeHeight();
        setTimeout(() => { // avoid flash of iframe background
          excalidrawBlock.classList.remove('is-loading');
          iframe.classList.remove('tw-invisible');
        }, 0);

        (new IntersectionObserver(() => {
          updateIframeHeight();
        }, {root: document.documentElement})).observe(iframe);
      });

      document.body.append(excalidrawBlock);
    } catch (err) {
      displayError(pre, err);
    }
  });
}
