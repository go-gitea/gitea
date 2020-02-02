export default async function initHighlight() {
  if (!window.config || !window.config.HighlightJS) return;

  const hljs = await import(/* webpackChunkName: "highlight" */'highlight.js');

  const nodes = [].slice.call(document.querySelectorAll('pre code') || []);
  for (let i = 0; i < nodes.length; i++) {
    hljs.highlightBlock(nodes[i]);
  }

  return hljs;
}
