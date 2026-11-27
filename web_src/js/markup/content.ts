import {initMarkupCodeMermaid} from './mermaid.ts';
import {initMarkupCodeMath} from './math.ts';
import {initMarkupCodeCopy} from './codecopy.ts';
import {initMarkupTasklist} from './tasklist.ts';
import {registerGlobalInitFunc, registerGlobalSelectorFunc} from '../modules/observer.ts';
import {initExternalRenderIframe} from './render-iframe.ts';
import {initMarkupCodeHighlight} from '../modules/highlight/dom.ts';

// code that runs for all markup content
export function initMarkupContent(): void {
  registerGlobalInitFunc('initExternalRenderIframe', initExternalRenderIframe);
  registerGlobalSelectorFunc('.markup', (el: HTMLElement) => {
    initMarkupCodeCopy(el);
    initMarkupTasklist(el);
    initMarkupCodeMermaid(el);
    initMarkupCodeMath(el);
    initMarkupCodeHighlight(el);
  });
}
