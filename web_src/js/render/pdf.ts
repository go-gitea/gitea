import {htmlEscape} from 'escape-goat';

export async function initPdfViewer() {
  const els = document.querySelectorAll('.pdf-content');
  if (!els.length) return;

  const pdfobject = await import(/* webpackChunkName: "pdfobject" */'pdfobject');

  for (const el of els) {
    const src = el.getAttribute('data-src');
    const fallbackText = el.getAttribute('data-fallback-button-text');
    pdfobject.embed(src, el, {
      fallbackLink: htmlEscape`
        <a role="button" class="ui basic button pdf-fallback-button" href="[url]">${fallbackText}</a>
      `,
    });
    el.classList.remove('is-loading');
  }
}
