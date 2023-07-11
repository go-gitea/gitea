import {isDarkTheme} from '../utils.js';
import {makeCodeCopyButton} from './codecopy.js';
import {displayError} from './common.js';

const {mermaidMaxSourceCharacters} = window.config;

const iframeCss = `:root {color-scheme: normal}
body {margin: 0; padding: 0; overflow: hidden}
#mermaid {display: block; margin: 0 auto}`;

export async function renderMermaid() {
  const els = document.querySelectorAll('.markup code.language-mermaid');
  if (!els.length) return;

  const {default: mermaid} = await import(/* webpackChunkName: "mermaid" */'mermaid');

  mermaid.initialize({
    startOnLoad: false,
    theme: isDarkTheme() ? 'dark' : 'neutral',
    securityLevel: 'strict',
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
      iframe.classList.add('markup-render', 'gt-invisible');
      iframe.srcdoc = `<html><head><style>${iframeCss}</style></head><body>${svg}</body></html>`;

      const mermaidBlock = document.createElement('div');
      mermaidBlock.classList.add('mermaid-block', 'is-loading', 'gt-hidden');
      mermaidBlock.append(iframe);

      const btn = makeCodeCopyButton();
      btn.setAttribute('data-clipboard-text', source);
      mermaidBlock.append(btn);

      iframe.addEventListener('load', () => {
        pre.replaceWith(mermaidBlock);
        mermaidBlock.classList.remove('gt-hidden');
        iframe.style.height = `${iframe.contentWindow.document.body.clientHeight}px`;
        setTimeout(() => { // avoid flash of iframe background
          mermaidBlock.classList.remove('is-loading');
          iframe.classList.remove('gt-invisible');
        }, 0);
      });

      document.body.append(mermaidBlock);
    } catch (err) {
      displayError(pre, err);
    }
  }
}
