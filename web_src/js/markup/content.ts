import {initMarkupCodeMermaid} from './mermaid.ts';
import {initMarkupCodeMath} from './math.ts';
import {initMarkupCodeCopy} from './codecopy.ts';
import {initMarkupRenderAsciicast} from './asciicast.ts';
import {initMarkupTasklist} from './tasklist.ts';
import {registerGlobalSelectorFunc} from '../modules/observer.ts';
import {initMarkupRenderIframe} from './render-iframe.ts';
import {initMarkupRefIssue} from './refissue.ts';
import {toggleElemClass} from '../utils/dom.ts';

// code that runs for all markup content
export function initMarkupContent(): void {
  registerGlobalSelectorFunc('.markup', (el: HTMLElement) => {
    if (el.matches('.truncated-markup')) {
      // when the rendered markup is truncated (e.g.: user's home activity feed)
      // we should not initialize any of the features (e.g.: code copy button), due to:
      // * truncated markup already means that the container doesn't want to show complex contents
      // * truncated markup may contain incomplete HTML/mermaid elements
      // so the only thing we need to do is to remove the "is-loading" class added by the backend render.
      toggleElemClass(el.querySelectorAll('.is-loading'), 'is-loading', false);
      return;
    }
    initMarkupCodeCopy(el);
    initMarkupTasklist(el);
    initMarkupCodeMermaid(el);
    initMarkupCodeMath(el);
    initMarkupRenderAsciicast(el);
    initMarkupRenderIframe(el);
    initMarkupRefIssue(el);
  });
}
