import {renderMermaid} from './mermaid.js';
import {initMarkupTasklist} from './tasklist.js';

// code that runs for all markup content
export async function initMarkupContent() {
  await renderMermaid(document.querySelectorAll('code.language-mermaid'));
}

// code that only runs for comments
export function initCommentContent() {
  initMarkupTasklist();
}
