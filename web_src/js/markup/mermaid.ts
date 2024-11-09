import {isDarkTheme} from '../utils.ts';
import {makeCodeCopyButton} from './codecopy.ts';
import {displayError} from './common.ts';

const {mermaidMaxSourceCharacters} = window.config;

// margin removal is for https://github.com/mermaid-js/mermaid/issues/4907
const iframeCss = `:root {color-scheme: normal}
body {margin: 0; padding: 0; overflow: hidden}
#mermaid {display: block; margin: 0 auto}
blockquote, dd, dl, figure, h1, h2, h3, h4, h5, h6, hr, p, pre {margin: 0}`;

export async function renderMermaid() {
  const els = document.querySelectorAll('.markup code.language-mermaid');
  if (!els.length) return;

  const {default: mermaid} = await import(/* webpackChunkName: "mermaid" */'mermaid');

  mermaid.initialize({
    startOnLoad: false,
    theme: isDarkTheme() ? 'dark' : 'neutral',
    securityLevel: 'strict',
    suppressErrorRendering: true,
  });

  for (const el of els) {
    const pre = el.closest('pre');
    if (pre.hasAttribute('data-render-done')) continue;

    const source = el.textContent;
    if (mermaidMaxSourceCharacters >= 0 && source.length > mermaidMaxSourceCharacters) {
      displayError(pre, new Error(`Mermaid source of ${source.length} characters exceeds the maximum allowed length of ${mermaidMaxSourceCharacters}.`));
      continue;
    }

    try {
      await mermaid.parse(source);
    } catch (err) {
      displayError(pre, err);
      continue;
    }

    try {
      // can't use bindFunctions here because we can't cross the iframe boundary. This
      // means js-based interactions won't work but they aren't intended to work either
      const {svg} = await mermaid.render('mermaid', source);

      const iframe = document.createElement('iframe');
      iframe.classList.add('markup-render', 'tw-invisible');
      iframe.srcdoc = `<html><head><style>${iframeCss}</style></head><body>${svg}</body></html>`;

      const mermaidBlock = document.createElement('div');
      mermaidBlock.classList.add('mermaid-block', 'is-loading', 'tw-hidden');
      mermaidBlock.append(iframe);

      const btn = makeCodeCopyButton();
      btn.setAttribute('data-clipboard-text', source);
      mermaidBlock.append(btn);

      const updateIframeHeight = () => {
        iframe.style.height = `${iframe.contentWindow.document.body.clientHeight}px`;
      };

      // update height when element's visibility state changes, for example when the diagram is inside
      // a <details> + <summary> block and the <details> block becomes visible upon user interaction, it
      // would initially set a incorrect height and the correct height is set during this callback.
      (new IntersectionObserver(() => {
        updateIframeHeight();
      }, {root: document.documentElement})).observe(iframe);

      iframe.addEventListener('load', () => {
        pre.replaceWith(mermaidBlock);
        mermaidBlock.classList.remove('tw-hidden');
        updateIframeHeight();
        setTimeout(() => { // avoid flash of iframe background
          mermaidBlock.classList.remove('is-loading');
          iframe.classList.remove('tw-invisible');
        }, 0);
      });

      document.body.append(mermaidBlock);
    } catch (err) {
      displayError(pre, err);
    }
  }
}
