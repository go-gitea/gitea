import {isDarkTheme} from '../utils.js';
const {mermaidMaxSourceCharacters} = window.config;

const iframeCss = `
  body {margin: 0; padding: 0}
  #mermaid {display: block; margin: 0 auto}
`;

function displayError(el, err) {
  el.closest('pre').classList.remove('is-loading');
  const errorNode = document.createElement('div');
  errorNode.setAttribute('class', 'ui message error markup-block-error mono');
  errorNode.textContent = err.str || err.message || String(err);
  el.closest('pre').before(errorNode);
}

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
    const source = el.textContent;

    if (mermaidMaxSourceCharacters >= 0 && source.length > mermaidMaxSourceCharacters) {
      displayError(el, new Error(`Mermaid source of ${source.length} characters exceeds the maximum allowed length of ${mermaidMaxSourceCharacters}.`));
      continue;
    }

    let valid;
    try {
      valid = mermaid.parse(source);
    } catch (err) {
      displayError(el, err);
    }

    if (!valid) {
      el.closest('pre').classList.remove('is-loading');
      continue;
    }

    try {
      // Can't use bindFunctions here because we can't pass functions to the iframe, which
      // means js-based interaction in charts will not work, but it seems this feature is
      // disabled in "strict" securityLevel anyways.
      mermaid.mermaidAPI.render('mermaid', source, (svgStr) => {
        const heightStr = (svgStr.match(/height="(.+?)"/) || [])[1];
        const height = heightStr ? Math.ceil(parseFloat(heightStr)) : 600; // best-effort fallback
        const iframe = document.createElement('iframe');
        iframe.classList.add('markup-render');
        iframe.sandbox = 'allow-scripts';
        iframe.scrolling = 'no';
        iframe.style.height = `${height}px`;
        iframe.srcdoc = `<html><head><style>${iframeCss}</style></head><body>${svgStr}</body></html>`;
        el.closest('pre').replaceWith(iframe);
      });
    } catch (err) {
      displayError(el, err);
    }
  }
}
