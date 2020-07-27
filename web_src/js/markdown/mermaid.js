import {random} from '../utils.js';

export async function renderMermaid(els) {
  if (!els || !els.length) return;

  const {mermaidAPI} = await import(/* webpackChunkName: "mermaid" */'mermaid');

  mermaidAPI.initialize({
    startOnLoad: false,
    theme: 'neutral',
    securityLevel: 'strict',
  });

  for (const el of els) {
    mermaidAPI.render(`mermaid-${random(12)}`, el.textContent, (svg, bindFunctions) => {
      const div = document.createElement('div');
      div.classList.add('mermaid-chart');
      div.innerHTML = svg;
      if (typeof bindFunctions === 'function') bindFunctions(div);
      el.closest('pre').replaceWith(div);
    });
  }
}
