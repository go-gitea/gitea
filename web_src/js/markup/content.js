import {renderMermaid} from './mermaid.js';
import {initMarkupTasklist} from './tasklist.js';

// code that runs for all markup content
export function initMarkupContent() {
  const _promise = renderMermaid(document.querySelectorAll('code.language-mermaid'));
}

// code that only runs for comments
export function initCommentContent() {
  initMarkupTasklist();
}
