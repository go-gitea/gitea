import {htmlEscape} from 'escape-goat';
import {registerGlobalInitFunc} from '../modules/observer.ts';

export async function initPdfViewer() {
  registerGlobalInitFunc('initPdfViewer', async (el: HTMLInputElement) => {
    const pdfobject = await import(/* webpackChunkName: "pdfobject" */'pdfobject');

    const src = el.getAttribute('data-src');
    const fallbackText = el.getAttribute('data-fallback-button-text');
    pdfobject.embed(src, el, {
      fallbackLink: htmlEscape`
        <a role="button" class="ui basic button pdf-fallback-button" href="[url]">${fallbackText}</a>
      `,
    });
    el.classList.remove('is-loading');
  });
}
