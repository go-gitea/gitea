import {initMarkupCodeMermaid} from './mermaid.ts';
import {initMarkupCodeMath} from './math.ts';
import {initMarkupCodeCopy} from './codecopy.ts';
import {initMarkupRenderAsciicast} from './asciicast.ts';
import {initMarkupTasklist} from './tasklist.ts';
import {registerGlobalSelectorFunc} from '../modules/observer.ts';
import {initMarkupRenderIframe} from './render-iframe.ts';
import {initMarkupRefIssue} from './refissue.ts';

// code that runs for all markup content
export function initMarkupContent(): void {
  registerGlobalSelectorFunc('.markup', (el: HTMLElement) => {
    initMarkupCodeCopy(el);
    initMarkupTasklist(el);
    initMarkupCodeMermaid(el);
    initMarkupCodeMath(el);
    initMarkupRenderAsciicast(el);
    initMarkupRenderIframe(el);
    initMarkupRefIssue(el);
  });
}
