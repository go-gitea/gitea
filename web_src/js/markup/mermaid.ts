import {isDarkTheme} from '../utils.ts';
import {makeCodeCopyButton} from './codecopy.ts';
import {displayError} from './common.ts';
import {queryElems} from '../utils/dom.ts';
import {html, htmlRaw} from '../utils/html.ts';

const {mermaidMaxSourceCharacters} = window.config;

const iframeCss = `:root {color-scheme: normal}
body {margin: 0; padding: 0; overflow: hidden}
#mermaid {display: block; margin: 0 auto}`;

/** detect whether mermaid sources contain elk layout configuration */
export function sourcesContainElk(sources: Array<string>) {
  return sources.some((source) => {
    return /(layout|defaultRenderer)[\s\S]*elk/.test(source);
  });
}

async function loadMermaid(sources: Array<string>) {
  const mermaidPromise = import(/* webpackChunkName: "mermaid" */'mermaid');
  const elkPromise = sourcesContainElk(sources) ?
    import(/* webpackChunkName: "mermaid-layout-elk" */'@mermaid-js/layout-elk') : null;

  const results = await Promise.all([mermaidPromise, elkPromise]);
  return {
    mermaid: results[0].default,
    elkLayouts: results[1]?.default,
  };
}

let elkLayoutsRegistered = false;

export async function initMarkupCodeMermaid(elMarkup: HTMLElement): Promise<void> {
  // .markup code.language-mermaid
  const els = Array.from(queryElems(elMarkup, 'code.language-mermaid'));
  if (!els.length) return;
  const sources = Array.from(els, (el) => el.textContent ?? '');
  const {mermaid, elkLayouts} = await loadMermaid(sources);

  for (const [index, el] of els.entries()) {
    const source = sources[index];
    const pre = el.closest('pre');

    if (!pre || pre.hasAttribute('data-render-done')) {
      continue;
    }

    if (mermaidMaxSourceCharacters >= 0 && source.length > mermaidMaxSourceCharacters) {
      displayError(pre, new Error(`Mermaid source of ${source.length} characters exceeds the maximum allowed length of ${mermaidMaxSourceCharacters}.`));
      continue;
    }

    if (elkLayouts && !elkLayoutsRegistered) {
      mermaid.registerLayoutLoaders(elkLayouts);
      elkLayoutsRegistered = true;
    }
    mermaid.initialize({
      startOnLoad: false,
      theme: isDarkTheme() ? 'dark' : 'neutral',
      securityLevel: 'strict',
      suppressErrorRendering: true,
    });

    try {
      await mermaid.parse(source);
    } catch (err) {
      displayError(pre, err);
      return;
    }

    try {
      // can't use bindFunctions here because we can't cross the iframe boundary. This
      // means js-based interactions won't work but they aren't intended to work either
      const {svg} = await mermaid.render('mermaid', source);

      const iframe = document.createElement('iframe');
      iframe.classList.add('markup-content-iframe', 'tw-invisible');
      iframe.srcdoc = html`<html><head><style>${htmlRaw(iframeCss)}</style></head><body>${htmlRaw(svg)}</body></html>`;

      const mermaidBlock = document.createElement('div');
      mermaidBlock.classList.add('mermaid-block', 'is-loading', 'tw-hidden');
      mermaidBlock.append(iframe);

      const btn = makeCodeCopyButton();
      btn.setAttribute('data-clipboard-text', source);
      mermaidBlock.append(btn);

      const updateIframeHeight = () => {
        const body = iframe.contentWindow?.document?.body;
        if (body) {
          iframe.style.height = `${body.clientHeight}px`;
        }
      };

      iframe.addEventListener('load', () => {
        pre.replaceWith(mermaidBlock);
        mermaidBlock.classList.remove('tw-hidden');
        updateIframeHeight();
        setTimeout(() => { // avoid flash of iframe background
          mermaidBlock.classList.remove('is-loading');
          iframe.classList.remove('tw-invisible');
        }, 0);

        // update height when element's visibility state changes, for example when the diagram is inside
        // a <details> + <summary> block and the <details> block becomes visible upon user interaction, it
        // would initially set a incorrect height and the correct height is set during this callback.
        (new IntersectionObserver(() => {
          updateIframeHeight();
        }, {root: document.documentElement})).observe(iframe);
      });

      document.body.append(mermaidBlock);
    } catch (err) {
      displayError(pre, err);
    }
  }
}
